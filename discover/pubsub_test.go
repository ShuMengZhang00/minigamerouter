package discover

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
)

const (
	servicePattern = "testservice_%d"
	addrPattern    = "10.10.%d.%d"
)

var (
	endpoints = []string{"localhost:2379"}
)

func Test1(t *testing.T) {
	key := "thekey"
	value := "thevalue"
	pub := NewPublisher(endpoints, key, value)
	assert.Nil(t, pub.KeepAlive())
	sub := NewSubscriber(endpoints, key)
	t.Errorf("sub get values: %v", sub.Values())
	assert.Equal(t, value, sub.Values()[0])
}

func register(serviceNum int, instanceNum int) {
	for i := range serviceNum {
		service := fmt.Sprintf(servicePattern, i)
		for j := range instanceNum {
			addr := fmt.Sprintf(addrPattern, i, j)
			pub := NewPublisher(endpoints, service, addr)
			if err := pub.KeepAlive(); err != nil {
				log.Fatalf("keep alive fail %s", err.Error())
			}
		}
	}
}

func BenchmarkSubscribe(b *testing.B) {

}
