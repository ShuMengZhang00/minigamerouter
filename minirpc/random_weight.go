package minirpc

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

const WeightRandom = "weight_random"

func newBulider() balancer.Builder {
	return base.NewBalancerBuilder(WeightRandom, &weightRandomBuilder{},
		base.Config{HealthCheck: true})
}

func init() {
	balancer.Register(newBuilder())
}

type weightRandomBuilder struct {
}

func (*weightRandomBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}
	var scs []balancer.SubConn

	for subconn, sc := range info.ReadySCs {
		weight := 1
		w := sc.Address.Attributes.Value("weight")
		if w != nil {
			if i, ok := w.(int); ok {
				weight = i
			}
		}
		for i := 0; i < weight; i++ {
			scs = append(scs, subconn)
		}
	}

	return &randPicker{subConns: scs}
}
