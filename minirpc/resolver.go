package minirpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"gamerouter/discover"
	router "gamerouter/router/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"log"
	"strconv"
	"strings"
)

const (
	EtcdScheme     = "etcd"
	keyServerInfo  = "server_info"
	keyDialOptions = "options"
)

type etcdResolverBuilder struct {
}

func getDialOptions(target resolver.Target) (*dialOptions, error) {
	options := &dialOptions{}
	if len(target.URL.RawQuery) > 0 {
		var optionsStr string
		values := target.URL.Query()
		if len(values) > 0 {
			optionValues := values[optionsKey]
			if len(optionValues) > 0 {
				optionsStr = optionValues[0]
			}
		}
		if len(optionsStr) > 0 {
			value, err := base64.URLEncoding.DecodeString(optionsStr)
			if err != nil {
				return nil, fmt.Errorf("fail to decode %s, options %s: %w",
					target.URL.Opaque, optionsStr, err)
			}
			if err = json.Unmarshal(value, options); nil != err {
				return nil, fmt.Errorf("fail to unmarshal options %s: %w",
					string(value), err)
			}
		}
	}
	return options, nil
}

func parseHost(target string) (string, int, error) {
	splits := strings.Split(target, ":")
	if len(splits) > 2 {
		return "", 0, errors.New("error format host")
	}
	if len(splits) == 1 {
		return target, 0, nil
	}
	port, err := strconv.Atoi(splits[1])
	if err != nil {
		return "", 0, err
	}
	return splits[0], port, nil
}

func getNamespace(options *dialOptions) string {
	namespace := DefaultNamespace
	if len(options.Namespace) > 0 {
		namespace = options.Namespace
	}
	return namespace
}

func (b *etcdResolverBuilder) Build(
	target resolver.Target,
	cc resolver.ClientConn,
	_ resolver.BuildOptions,
) (resolver.Resolver, error) {
	options, err := getDialOptions(target)
	if err != nil {
		return nil, err
	}
	host, _, err := parseHost(target.URL.Host)
	if err != nil {
		return nil, err
	}
	if len(options.RouteKey) > 0 {
		conn, err := DialContext(context.Background(),
			"etcd://MiniRouter",
			WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
			WithEtcdHosts(options.Endpoints))
		if err != nil {
			fmt.Printf("[Resolver Builder] dial error in %v", err)
		}
		cli := router.NewRouterClient(conn)
		resolv := &dynamicPrefixResolver{
			routercli: cli, serviceName: host, namespace: options.Namespace,
		}
		resolv.update()
		return resolv, nil
	}
	sub := discover.NewSubscriber(options.Endpoints,
		MakeEtcdServiceKey(getNamespace(options), host))
	resolv := &namingResolver{
		cc:      cc,
		sub:     sub,
		options: options,
	}
	sub.AddListener(resolv.update)
	resolv.update()

	return resolv, nil
}

type dynamicPrefixResolver struct {
	cc          resolver.ClientConn
	routercli   router.RouterClient
	routeKey    string
	serviceName string
	namespace   string
}

func (d *dynamicPrefixResolver) update() {
	resp, err := d.routercli.GetOneInstanceWithPrefix(context.Background(),
		&router.GetEndpointWithPrefixRequest{
			ServiceName: d.serviceName,
			Namespace:   d.namespace, Key: d.routeKey,
		})
	if err != nil {
		fmt.Printf("dynamic prefix resolver err: %v", err)
	}
	addr := fmt.Sprintf("%s:%s", resp.Instance.Host, resp.Instance.Port)
	state := resolver.State{
		Addresses: make([]resolver.Address, 0),
	}
	state.Addresses = append(state.Addresses, resolver.Address{
		Addr: addr,
	})
	if err := d.cc.UpdateState(state); err != nil {
		fmt.Printf("dynamic resolver err %v", err)
	}
}

func (d *dynamicPrefixResolver) ResolveNow(_ resolver.ResolveNowOptions) {

}

func (d *dynamicPrefixResolver) Close() {
}

func (b *etcdResolverBuilder) Scheme() string {
	return EtcdScheme
}

type namingResolver struct {
	cc      resolver.ClientConn
	sub     *discover.Subscriber
	options *dialOptions
}

func (n *namingResolver) update() {
	vals := n.sub.Values()
	serverInfos := make([]ServerInfo, 0, len(vals))
	for _, val := range vals {
		info := &ServerInfo{}
		if err := json.Unmarshal([]byte(val), info); err != nil {
			fmt.Sprintf("error in %v", err)
		}
		serverInfos = append(serverInfos, *info)
	}

	// resolver state definition, pass it to balancer
	state := resolver.State{
		Attributes: attributes.New(keyDialOptions,
			n.options).WithValue(keyServerInfo, serverInfos),
	}
	for _, info := range serverInfos {
		state.Addresses = append(state.Addresses, resolver.Address{
			Addr: fmt.Sprintf("%s:%d", info.Host, info.Port),
		})
	}

	if err := n.cc.UpdateState(state); err != nil {
		log.Print(err)
	}
}

func (n *namingResolver) ResolveNow(_ resolver.ResolveNowOptions) {
}

func (n *namingResolver) Close() {
}
