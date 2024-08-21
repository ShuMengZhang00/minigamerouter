package minirpc

import "testing"

var endpoints = []string{
	"127.0.0.1:2379",
}

func TestPubToEtcd(t *testing.T) {
	info := ServerInfo{
		Namespace:      "test",
		ServiceName:    "mytest",
		Host:           "192.0.0.1",
		Port:           5000,
		InstanceID:     "haha",
		ServerMetadata: nil,
	}
	s, _ := NewServer(info)
	_ = s.pubToEtcd(endpoints, info)
}
