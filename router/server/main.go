package main

import (
	"context"
	"flag"
	"fmt"
	"gamerouter/discover"
	"gamerouter/minirpc"
	route "gamerouter/router"
	router "gamerouter/router/proto"
	"google.golang.org/grpc"
	"log"
	"net"
	"strconv"
)

var (
	RouterServiceName = "MiniRouter"
	endpoints         = []string{"127.0.0.1:2379"}
	address           = flag.String("addr", "localhost:50051", "listen addr")
)

type RouterService struct {
	endpoints []string

	routeTable      *route.RouteTable
	prefixRuleTable *route.RuleTable

	router.UnimplementedRouterServer
}

func NewRouterService(endpoints []string) *RouterService {
	r := &RouterService{
		endpoints:       endpoints,
		prefixRuleTable: route.NewRuleTable(endpoints),
		routeTable:      route.NewRouteTable(endpoints),
	}
	return r
}

func (r *RouterService) GetOneInstanceWithPrefix(ctx context.Context, request *router.GetEndpointWithPrefixRequest) (*router.GetOneInstanceResponse, error) {
	instanceID, ok := r.prefixRuleTable.GetInstanceID(request.Namespace,
		request.ServiceName, request.Key)
	if !ok {
		return nil, fmt.Errorf("no instance found")
	}
	info, ok := r.routeTable.GetServerInfo(request.Namespace,
		request.ServiceName, instanceID)
	if !ok {
		return nil, fmt.Errorf("the instance %s is off", instanceID)
	}
	resp := &router.GetOneInstanceResponse{
		Instance: &router.ServiceInfo{
			Namespace:   request.Namespace,
			ServiceName: request.ServiceName,
			InstanceId:  instanceID,
			Host:        info.Host,
			Port:        strconv.Itoa(info.Port),
			Weight:      int64(info.Weight),
			Metadata:    mapToKVs(info.ServerMetadata),
		},
		Error: "",
	}
	return resp, nil
}

func mapToKVs(metadata map[string]string) []*router.KV {
	var res []*router.KV
	for k, v := range metadata {
		res = append(res, &router.KV{
			Key:   k,
			Value: v,
		})
	}
	return res
}

func (r *RouterService) SetRouteRule(ctx context.Context, request *router.SetRouteRuleRequest) (*router.SetRouteRuleResponse, error) {
	key := getRouteRuleEtcdKey(request.Namespace, request.ServiceName,
		request.Prefix)
	_, err := discover.GetRegistry(r.endpoints).GetConn().Put(ctx, key,
		request.Prefix)
	if err != nil {
		return &router.SetRouteRuleResponse{ErrorMes: err.Error()}, err
	}
	return nil, nil
}

func getRouteRuleEtcdKey(namespace, servicename, prefix string) string {
	return fmt.Sprintf("%s/%s/%s/%s", route.RouteRulePrefix, namespace,
		servicename,
		prefix)
}

func (r *RouterService) mustEmbedUnimplementedRouterServer() {
	panic("implement me")
}

func main() {
	flag.Parse()
	listen, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatalf("failed to addr %s: %v", *address, err)
	}
	fmt.Printf("router is listening %s\n", listen.Addr().String())

	srv := grpc.NewServer()
	router.RegisterRouterServer(srv, NewRouterService(endpoints))
	if err = minirpc.Serve(srv, listen,
		minirpc.WithServerNamespace(minirpc.DefaultNamespace),
		minirpc.WithServiceName(RouterServiceName),
		minirpc.WithEtcdEndPoints(endpoints)); err != nil {
		log.Printf("lisen err: %v", err)
	}
}
