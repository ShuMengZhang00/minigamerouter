package minirpc

import (
	"fmt"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

const KetamaWeightName = "ketama_hash"

func init() {

}

type KetamaWeightHashPickerBuilder struct {
}

func (k KetamaWeightHashPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}
	picker := &KetamaWeightHashPicker{
		addr2subConns: make(map[string]balancer.SubConn),
	}
	nodes := make([]*Node, 0, len(info.ReadySCs))
	for subconn, subconninfo := range info.ReadySCs {
		weight := 1
		m := subconninfo.Address.Attributes.Value(NodeWeight)
		if m != nil {
			w, ok := m.(int)
			if ok {
				weight = w
			}
		}
		addr := subconninfo.Address.Addr
		nodes = append(nodes, NewNode(addr, uint(weight)))
		picker.addr2subConns[addr] = subconn
	}
	picker.ring = NewRing(nodes)
	return picker
}

type KetamaWeightHashPicker struct {
	addr2subConns map[string]balancer.SubConn
	ring          *Ring
}

func (p *KetamaWeightHashPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	key, ok := info.Ctx.Value(KetamaWeightName).(string)
	if !ok {
		return balancer.PickResult{}, fmt.Errorf("do not find ketama key")
	}
	addr := p.ring.Get(key).key
	return balancer.PickResult{SubConn: p.addr2subConns[addr]}, nil
}
