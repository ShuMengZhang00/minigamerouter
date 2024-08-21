package minirpc

import (
	"errors"
	"fmt"
	router "gamerouter/router/proto"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/resolver"
	"log"
	"math/rand"
	"sort"
	"stathat.com/c/consistent"
	"strconv"
	"sync"
	"time"
)

type balancerBuilder struct {
}

func (bb *balancerBuilder) Build(cc balancer.ClientConn, opts balancer.BuildOptions) balancer.Balancer {
	target := opts.Target
	host, _, _ := parseHost(target.URL.Host)
	return &namingBalancer{
		cc:          cc,
		target:      opts.Target,
		serviceName: host,
		subConns:    make(map[string]balancer.SubConn),
		scStates:    make(map[balancer.SubConn]connectivity.State),
		csEvltr:     &balancer.ConnectivityStateEvaluator{},
	}
}

func (bb *balancerBuilder) Name() string { return EtcdScheme }

type namingBalancer struct {
	cc          balancer.ClientConn
	target      resolver.Target
	serviceName string
	rwMutex     sync.RWMutex

	csEvltr *balancer.ConnectivityStateEvaluator
	state   connectivity.State

	subConns    map[string]balancer.SubConn
	scStates    map[balancer.SubConn]connectivity.State
	serverInfos []ServerInfo
	picker      balancer.Picker

	cons *consistent.Consistent

	dialOptions *dialOptions

	resolverErr error // the last error reported by the resolver; cleared on successful resolution
	connErr     error // the last connection error; cleared upon leaving TransientFailure
}

func (n *namingBalancer) createSubConnection(key string, addr resolver.Address) {
	n.rwMutex.Lock()
	defer n.rwMutex.Unlock()
	if _, ok := n.subConns[key]; ok {
		return
	}
	// new addr:
	sc, err := n.cc.NewSubConn(
		[]resolver.Address{addr},
		balancer.NewSubConnOptions{HealthCheckEnabled: false})
	if err != nil {
		fmt.Printf("balancer failed to create new SubConn: %v", err)
		return
	}
	n.subConns[key] = sc
	n.scStates[sc] = connectivity.Idle
	n.csEvltr.RecordTransition(connectivity.Shutdown, connectivity.Idle)
	sc.Connect()
}

// UpdateClientConnState is called by gRPC when the state of the ClientConn
// changes.  If the error returned is ErrBadResolverState, the ClientConn
// will begin calling ResolveNow on the active name resolver with
// exponential backoff until a subsequent call to UpdateClientConnState
// returns a nil error.  Any other errors are currently ignored.
func (n *namingBalancer) UpdateClientConnState(state balancer.ClientConnState) error {
	if n.dialOptions == nil && state.ResolverState.Attributes != nil {
		n.dialOptions = state.ResolverState.Attributes.Value(keyDialOptions).(*dialOptions)
	}
	if state.ResolverState.Attributes != nil {
		n.serverInfos = state.ResolverState.Attributes.Value(keyServerInfo).([]ServerInfo)
	}
	if len(state.ResolverState.Addresses) == 0 {
		log.Printf("balancer receive empty address, service name=%s",
			n.serviceName)
		n.ResolverError(errors.New("produced zero addresses"))
		return balancer.ErrBadResolverState
	}
	// resolution succeed
	n.resolverErr = nil
	addrSet := make(map[string]struct{})
	for _, a := range state.ResolverState.Addresses {
		key := fmt.Sprintf("%s", a.Addr)
		addrSet[key] = struct{}{}
		n.createSubConnection(key, a)
	}
	n.rwMutex.Lock()
	defer n.rwMutex.Unlock()
	for a, sc := range n.subConns {
		// a way removed by resolver.
		if _, ok := addrSet[a]; !ok {
			delete(n.subConns, a)
			sc.Shutdown()
			// Keep the state of this sc in b.scStates until sc's state becomes Shutdown.
			// The entry will be deleted in HandleSubConnStateChange.
		}
	}
	n.regeneratePicker(n.dialOptions)
	n.cc.UpdateState(balancer.State{
		ConnectivityState: n.state, Picker: n.picker,
	})
	return nil
}

func (n *namingBalancer) regeneratePicker(options *dialOptions) {
	if n.state == connectivity.TransientFailure {
		n.picker = base.NewErrPicker(n.mergeErrors())
		return
	}
	readySCs := make(map[string]balancer.SubConn)
	// Filter out all ready SCs from full subConn map.
	for addr, sc := range n.subConns {
		if st, ok := n.scStates[sc]; ok && st == connectivity.Ready {
			readySCs[addr] = sc
		}
	}

	readyInstances := make([]ServerInfo, 0, len(readySCs))
	readyAddr := make([]string, 0, len(readySCs))

	preWeight := make([]int, 0, len(readySCs))
	totalWeight := 0
	for _, instance := range n.serverInfos {
		// see buildAddressKey
		key := instance.Host + ":" + strconv.FormatInt(int64(instance.Port),
			10)
		if _, ok := readySCs[key]; ok {
			readyInstances = append(readyInstances, instance)
			readyAddr = append(readyAddr, key)

			preWeight = append(preWeight, totalWeight)
			totalWeight += instance.Weight
		}
	}

	picker := &namingPicker{
		balancer:    n,
		readySCs:    readySCs,
		options:     options,
		serverInfos: readyInstances,
		preWeight:   preWeight,
	}
	if options.LbPolicy == KetamaWeightName {
		if n.cons == nil {
			n.cons = consistent.New()
		}
		n.cons.Set(readyAddr)
		picker.Cons = n.cons
	}
	n.picker = picker
}

// mergeErrors builds an error from the last connection error and the last resolver error.
// It Must only be called if the b.state is TransientFailure.
func (n *namingBalancer) mergeErrors() error {
	// connErr must always be non-nil unless there are no SubConns, in which
	// case resolverErr must be non-nil.
	if n.connErr == nil {
		return fmt.Errorf("last resolver error: %w", n.resolverErr)
	}
	if n.resolverErr == nil {
		return fmt.Errorf("last connection error: %w", n.connErr)
	}
	return fmt.Errorf("last connection error: %v; last resolver error: %v",
		n.connErr, n.resolverErr)
}

// ResolverError is called by gRPC when the name resolver reports an error.
func (n *namingBalancer) ResolverError(err error) {
	n.resolverErr = err
	if len(n.subConns) == 0 {
		n.state = connectivity.TransientFailure
	}
	if n.state != connectivity.TransientFailure {
		return
	}
	n.rwMutex.RLock()
	defer n.rwMutex.RUnlock()
	n.cc.UpdateState(balancer.State{
		ConnectivityState: n.state,
		Picker:            n.picker,
	})
}

// UpdateSubConnState is called by gRPC when the state of a SubConn changes.
func (n *namingBalancer) UpdateSubConnState(conn balancer.SubConn, state balancer.SubConnState) {
	s := state.ConnectivityState
	n.rwMutex.Lock()
	defer n.rwMutex.Unlock()
	oldS, ok := n.scStates[conn]
	if !ok {
		fmt.Printf("[Polaris][Balancer] got state changes for an unknown SubConn: %p, %v",
			conn, s)
		return
	}
	if oldS == connectivity.TransientFailure && (s == connectivity.Connecting || s == connectivity.Idle) {
		if s == connectivity.Idle {
			conn.Connect()
		}
		return
	}
	n.scStates[conn] = s
	switch s {
	case connectivity.Idle:
		conn.Connect()
	case connectivity.Shutdown:
		// When an address was removed by resolver, b called RemoveSubConn but
		// kept the sc's state in scStates. Remove state for this sc here.
		delete(n.scStates, conn)
	case connectivity.TransientFailure:
		// Save error to be reported via picker.
		n.connErr = state.ConnectionError
	}
	n.state = n.csEvltr.RecordTransition(oldS, s)

	// Regenerate picker when one of the following happens:
	//  - this sc entered or left ready
	//  - the aggregated state of balancer is TransientFailure
	//    (may need to update error message)
	if (s == connectivity.Ready) != (oldS == connectivity.Ready) ||
		n.state == connectivity.TransientFailure {
		n.regeneratePicker(n.dialOptions)
	}

	n.cc.UpdateState(balancer.State{
		ConnectivityState: n.state, Picker: n.picker,
	})
}

func (n *namingBalancer) Close() {
}

type namingPicker struct {
	balancer    *namingBalancer
	readySCs    map[string]balancer.SubConn
	options     *dialOptions
	serverInfos []ServerInfo
	// 权重的前缀和，用于带权重的随机算法
	preWeight []int
	Cons      *consistent.Consistent

	routerAPI *router.RouterClient
}

func (p *namingPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	if len(p.serverInfos) < 1 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	if len(p.serverInfos) == 1 {
		sc, ok := p.getSubConns(p.serverInfos[0])
		if !ok {
			return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
		}
		return balancer.PickResult{
			SubConn: sc,
		}, nil
	}
	md, ok := metadata.FromOutgoingContext(info.Ctx)
	lbPolicy := p.options.LbPolicy
	var lbHashKey string
	if ok {
		lbPolicyValues := md.Get(miniRequestLbPolicy)
		lbHashKeyValues := md.Get(miniRequestLbHashKey)

		if len(lbPolicyValues) > 0 {
			lbPolicy = lbPolicyValues[0]
		}
		if len(lbHashKeyValues) > 0 {
			lbHashKey = lbHashKeyValues[0]
		}
	}
	switch lbPolicy {
	case Random:
		return p.pickRandom()
	case WeightRandom:
		return p.pickWeightRandom()
	case KetamaWeightName:
		return p.pickKetamaWeightRandom(lbHashKey)
	}
	return p.pickRandom()
}
func (p *namingPicker) pickRandom() (balancer.PickResult, error) {
	rand.NewSource(time.Now().UnixNano())
	scLen := len(p.serverInfos)
	info := p.serverInfos[rand.Intn(scLen)]
	subconn, ok := p.getSubConns(info)
	if !ok {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	return balancer.PickResult{SubConn: subconn}, nil
}

func (p *namingPicker) pickWeightRandom() (balancer.PickResult, error) {
	bound := p.preWeight[len(p.preWeight)-1]
	if bound <= 0 {
		return p.pickRandom()
	}
	x := rand.Intn(bound) + 1
	index := sort.SearchInts(p.preWeight, x)
	subconn, _ := p.getSubConns(p.serverInfos[index])
	return balancer.PickResult{SubConn: subconn}, nil
}

func (p *namingPicker) pickKetamaWeightRandom(hashKey string) (balancer.PickResult, error) {
	if p.Cons == nil {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	addr, err := p.Cons.Get(hashKey)
	if err != nil {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	if res, ok := p.readySCs[addr]; ok {
		return balancer.PickResult{SubConn: res}, nil
	}
	return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
}

func (p *namingPicker) getSubConns(info ServerInfo) (balancer.SubConn, bool) {
	addr := fmt.Sprintf("%s:%d", info.Host, info.Port)
	subconn, ok := p.readySCs[addr]
	return subconn, ok
}
