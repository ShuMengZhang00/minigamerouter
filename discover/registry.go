package discover

import (
	"context"
	"errors"
	"fmt"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	EtcdPathDelimiter = '/'

	autoSyncInterval   = time.Minute
	dialTimeout        = 5 * time.Second
	endPointsSeparator = ","
)

var (
	manager = RegistryManager{
		registries: make(map[string]*Registry),
	}
)

const (
	requestTimeout = 3 * time.Second
)

type RegistryManager struct {
	registries map[string]*Registry
	lock       sync.Mutex
}

func GetRegistry(endpoints []string) *Registry {
	key := getKey(endpoints)
	registry, ok := manager.registries[key]
	if ok {
		return registry
	}
	manager.lock.Lock()
	defer manager.lock.Unlock()
	if _, ok = manager.registries[key]; !ok {
		var err error
		manager.registries[key], err = newRegistry(endpoints)
		if err == nil {
			return manager.registries[key]
		}
	}
	return manager.registries[key]
}

func getKey(endpoints []string) string {
	sort.Strings(endpoints)
	return strings.Join(endpoints, endPointsSeparator)
}

type Registry struct {
	client     *clientv3.Client
	values     map[string]map[string]string // prefix->key->value
	listeners  map[string][]UpdateListener
	watchGroup sync.WaitGroup
	done       chan struct{}
	lock       sync.RWMutex
}

func newRegistry(endpoints []string) (*Registry, error) {
	cfg := clientv3.Config{
		Endpoints:           endpoints,
		AutoSyncInterval:    autoSyncInterval,
		DialTimeout:         dialTimeout,
		RejectOldCluster:    true,
		PermitWithoutStream: true,
	}
	// todo: account?
	cli, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	return &Registry{
		client:    cli,
		values:    make(map[string]map[string]string),
		listeners: make(map[string][]UpdateListener),
		done:      make(chan struct{}),
	}, nil
}

func (r *Registry) GetConn() *clientv3.Client {
	return r.client
}

func (r *Registry) Monitor(
	key string,
	l UpdateListener,
) {
	kvs := r.getCurrent(key)
	for _, kv := range kvs {
		l.OnAdd(kv)
	}

	r.lock.Lock()
	r.listeners[key] = append(r.listeners[key], l)
	r.lock.Unlock()

	rev := r.load(key)

	go func() {
		r.watch(key, rev)
	}()
}

func (r *Registry) load(prefix string) int64 {
	var resp *clientv3.GetResponse
	for {
		var err error
		ctx, cancel := context.WithTimeout(r.client.Ctx(), requestTimeout)
		resp, err = r.client.Get(ctx, prefix,
			clientv3.WithPrefix())
		cancel()
		if err == nil {
			break
		}
		log.Printf("%s, prefix is %s", err.Error(), prefix)
		time.Sleep(time.Second)
	}
	var kvs []KV
	for _, ev := range resp.Kvs {
		kvs = append(kvs, KV{
			Key: string(ev.Key),
			Val: string(ev.Value),
		})
	}

	r.handleChanges(prefix, kvs)

	return resp.Header.Revision
}

func (r *Registry) handleChanges(key string, kvs []KV) {
	var add []KV
	var remove []KV

	r.lock.Lock()
	listeners := append([]UpdateListener(nil), r.listeners[key]...)
	vals, ok := r.values[key]
	if !ok {
		add = kvs
		vals = make(map[string]string)
		// all value are new
		for _, kv := range kvs {
			vals[kv.Key] = kv.Val
		}
		r.values[key] = vals
	} else {
		m := make(map[string]string)
		for _, kv := range kvs {
			m[kv.Key] = kv.Val
		}
		// compute removed values
		for k, v := range vals {
			if val, ok := m[k]; !ok || v != val {
				remove = append(remove, KV{
					Key: k,
					Val: v,
				})
			}
		}
		// compute new values
		for k, v := range m {
			if val, ok := vals[k]; !ok || v != val {
				add = append(add, KV{
					Key: k,
					Val: v,
				})
			}
		}
	}
	r.lock.Unlock()

	for _, l := range listeners {
		for _, kv := range add {
			l.OnAdd(kv)
		}
		for _, kv := range remove {
			l.OnDelete(kv)
		}
	}
}

func (r *Registry) watch(key string, rev int64) {
	for {
		err := r.watchStream(key, rev)
		if err == nil {
			return
		}

		if rev != 0 && errors.Is(err, rpctypes.ErrCompacted) {
			log.Printf("etcd compacted, try to reload, rev %d", rev)
			rev = r.load(key)
		}

		log.Printf(err.Error())
	}
}

func (r *Registry) watchStream(key string, rev int64) error {
	var rch clientv3.WatchChan
	if rev != 0 {
		rch = r.client.Watch(clientv3.WithRequireLeader(r.client.Ctx()),
			makeKeyPrefix(key),
			clientv3.WithPrefix(), clientv3.WithRev(rev+1))
	} else {
		rch = r.client.Watch(clientv3.WithRequireLeader(r.client.Ctx()),
			makeKeyPrefix(key),
			clientv3.WithPrefix())
	}

	for {
		select {
		case wresp, ok := <-rch:
			if !ok {
				return fmt.Errorf("etcd monitor chan closed")
			}
			if wresp.Canceled {
				return fmt.Errorf("etcd monitor chan has been canceled, error: %w",
					wresp.Err())
			}
			if wresp.Err() != nil {
				return fmt.Errorf("etcd monitor chan error: %w", wresp.Err())
			}
			r.handleWatchEvents(key, wresp.Events)
		case <-r.done:
			return nil
		}
	}
}

func (r *Registry) handleWatchEvents(key string, events []*clientv3.Event) {
	r.lock.RLock()
	listeners := append([]UpdateListener(nil), r.listeners[key]...)
	r.lock.RUnlock()

	for _, ev := range events {
		newKey := string(ev.Kv.Key)
		newValue := string(ev.Kv.Value)
		switch ev.Type {
		case clientv3.EventTypePut:
			r.lock.Lock()
			if vals, ok := r.values[key]; ok {
				vals[newKey] = newValue
			} else {
				r.values[key] = map[string]string{newKey: newValue}
			}
			r.lock.Unlock()
			for _, l := range listeners {
				l.OnAdd(KV{
					Key: newKey,
					Val: newValue,
				})
			}
		case clientv3.EventTypeDelete:
			r.lock.Lock()
			if vals, ok := r.values[key]; ok {
				delete(vals, newKey)
			}
			r.lock.Unlock()
			for _, l := range listeners {
				l.OnDelete(KV{
					Key: newKey,
					Val: newValue,
				})
			}
		default:
			log.Printf("unknown event type: %v", ev.Type)
		}
	}
}

func (r *Registry) getCurrent(key string) []KV {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var kvs []KV
	for k, v := range r.values[key] {
		kvs = append(kvs, KV{
			Key: k,
			Val: v,
		})
	}
	return kvs
}

func makeKeyPrefix(key string) string {
	return fmt.Sprintf("%s%c", key, EtcdPathDelimiter)
}
