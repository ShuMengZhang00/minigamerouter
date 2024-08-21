package minirpc

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"math/rand"
	"time"
)

const Random = "random"

func init() {
	balancer.Register(newBuilder())
}

func newBuilder() balancer.Builder {
	return base.NewBalancerBuilder(Random, &randPickerBuilder{},
		base.Config{HealthCheck: true})
}

type randPickerBuilder struct {
}

func (b randPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}
	scs := make([]balancer.SubConn, 0, len(info.ReadySCs))
	for sc := range info.ReadySCs {
		scs = append(scs, sc)
	}
	return &randPicker{subConns: scs}
}

type randPicker struct {
	subConns []balancer.SubConn
}

func (p *randPicker) Pick(_ balancer.PickInfo) (balancer.PickResult, error) {
	rand.NewSource(time.Now().UnixNano())
	scLen := len(p.subConns)
	sc := p.subConns[rand.Intn(scLen)]
	return balancer.PickResult{SubConn: sc}, nil
}
