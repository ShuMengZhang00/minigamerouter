package main

import (
	"context"
	"encoding/json"
	"fmt"
	"gamerouter/discover"
	"gamerouter/minirpc"
	router "gamerouter/router/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
)

var (
	endpoints = []string{"127.0.0.1:2379"}
	charSet   = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
)

func genPrefixStrings(n int) []string {
	var allStrings []string
	var generate func(int, string)
	generate = func(pos int, current string) {
		if pos == n {
			allStrings = append(allStrings, current)
			return
		}

		for _, char := range charSet {
			generate(pos+1, current+string(char))
		}
	}
	generate(0, "")
	return allStrings
}

func main() {
	conn, err := minirpc.DialContext(context.Background(),
		"etcd://MiniRouter",
		minirpc.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
		minirpc.WithLoadBalancer(minirpc.Random),
		minirpc.WithEtcdHosts(endpoints))
	if err != nil {
		log.Fatal(err)
	}
	routercli := router.NewRouterClient(conn)
	servicename := "testserver"
	prefixs := genPrefixStrings(4)
	for _, p := range prefixs {
		instanceKey := minirpc.MakeEtcdInstanceKey(minirpc.DefaultNamespace,
			servicename, p)
		info := minirpc.ServerInfo{
			Namespace:      minirpc.DefaultNamespace,
			ServiceName:    servicename,
			InstanceID:     p,
			Weight:         1,
			Host:           "0.0.0.1",
			Port:           0,
			ServerMetadata: nil,
		}
		val, _ := json.Marshal(info)
		if _, err := discover.GetRegistry(endpoints).GetConn().Put(context.Background(),
			instanceKey, string(val)); err != nil {
			fmt.Println(err.Error())
		}

		_, err = routercli.SetRouteRule(context.Background(),
			&router.SetRouteRuleRequest{
				Namespace:   minirpc.DefaultNamespace,
				ServiceName: servicename,
				InstanceID:  p,
				Prefix:      p,
			})
		if err != nil {
			log.Fatal(err)
		}
	}

}
