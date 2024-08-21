package discover

import (
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"log"
	"time"
)

type (
	PubOption func(client *Publisher)

	Publisher struct {
		endpoints []string
		fullKey   string
		id        int64
		key       string
		value     string
		lease     clientv3.LeaseID
		quit      chan struct{}
	}
)

const TimeToLive int64 = 10

// invoke KeepAlive to keep key:value alive

func NewPublisher(
	endpoints []string,
	key, value string,
	opts ...PubOption,
) *Publisher {
	publisher := &Publisher{
		endpoints: endpoints,
		key:       key,
		value:     value,
		quit:      make(chan struct{}),
	}

	for _, opt := range opts {
		opt(publisher)
	}

	return publisher
}

func WithId(id int64) PubOption {
	return func(publisher *Publisher) {
		publisher.id = id
	}
}

// KeepAlive keep key:value alive
func (p *Publisher) KeepAlive() error {
	cli, err := p.doRegister()
	if err != nil {
		return err
	}

	return p.keepAliveAsync(cli)
}

func (p *Publisher) Stop() {
	close(p.quit)
}

func (p *Publisher) keepAliveAsync(cli *clientv3.Client) error {
	ch, err := cli.Lease.KeepAlive(cli.Ctx(), p.lease)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case _, ok := <-ch:
				if !ok {
					p.revoke(cli)
					// 重新注册
					if err := p.doKeepAlive(); err != nil {
						log.Printf("etcd publisher KeepAlive: %s", err.Error())
					}
					return
				}
			case <-p.quit:
				p.revoke(cli)
				return
			}
		}
	}()
	return nil
}

func (p *Publisher) doKeepAlive() error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case <-p.quit:
			return nil
		default:
			cli, err := p.doRegister()
			if err != nil {
				log.Printf("etcd publisher doRegister: %s", err.Error())
				break
			}

			if err := p.keepAliveAsync(cli); err != nil {
				log.Printf("etcd publisher keepAliveAsync: %s", err.Error())
				break
			}
			return nil
		}
	}

	return nil
}

func (p *Publisher) revoke(cli *clientv3.Client) {
	if _, err := cli.Revoke(cli.Ctx(), p.lease); err != nil {
		log.Printf("etcd publisher revoke error: %s", err.Error())
	}
}

func (p *Publisher) doRegister() (*clientv3.Client, error) {
	cli := GetRegistry(p.endpoints).GetConn()
	if cli == nil {
		return nil, fmt.Errorf("no client for %v", p.endpoints)
	}
	// register kv
	resp, err := cli.Grant(cli.Ctx(), TimeToLive)
	if err != nil {
		return nil, err
	}
	p.lease = resp.ID

	_, err = cli.Put(cli.Ctx(), p.key, p.value, clientv3.WithLease(p.lease))

	return cli, err
}
