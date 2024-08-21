package discover

import (
	"sync"
	"sync/atomic"
)

type (
	SubOption func(sub *Subscriber)

	Subscriber struct {
		endpoints []string
		mapping   map[string]string
		snapshot  atomic.Value
		dirty     atomic.Bool
		listeners []func() // notify updates
		lock      sync.Mutex
	}
)

func NewSubscriber(endpoints []string, key string, opts ...SubOption) *Subscriber {
	sub := &Subscriber{
		endpoints: endpoints,
		mapping:   make(map[string]string),
	}
	for _, opt := range opts {
		opt(sub)
	}
	GetRegistry(endpoints).Monitor(key, sub)

	return sub
}

func (s *Subscriber) AddListener(listener func()) {
	s.lock.Lock()
	s.listeners = append(s.listeners, listener)
	s.lock.Unlock()
}

func (s *Subscriber) notifyChange() {
	s.lock.Lock()
	listeners := append(([]func())(nil), s.listeners...)
	s.lock.Unlock()
	for _, listener := range listeners {
		listener()
	}
}

func (s *Subscriber) OnAdd(kv KV) {
	s.lock.Lock()
	s.dirty.Store(true)
	s.mapping[kv.Key] = kv.Val
	s.lock.Unlock()
	s.notifyChange()
}

func (s *Subscriber) OnDelete(kv KV) {
	s.lock.Lock()
	s.dirty.Store(true)
	_, ok := s.mapping[kv.Key]
	if !ok {
		s.lock.Unlock()
		return
	}
	delete(s.mapping, kv.Key)
	s.lock.Unlock()
	s.notifyChange()
}

func (s *Subscriber) KeyValues() map[string]string {
	if !s.dirty.Load() {
		if m, ok := s.snapshot.Load().(map[string]string); ok {
			return m
		}
		return map[string]string{}
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.snapshot.Store(s.mapping)
	s.dirty.Store(false)

	return s.mapping
}

func (s *Subscriber) Values() []string {
	if !s.dirty.Load() {
		if m, ok := s.snapshot.Load().(map[string]string); ok {
			return toSlice(m)
		}
		return []string{}
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	vals := toSlice(s.mapping)
	s.snapshot.Store(s.mapping)
	s.dirty.Store(false)

	return vals
}

func toSlice(m map[string]string) []string {
	var vals []string
	for _, v := range m {
		vals = append(vals, v)
	}
	return vals
}
