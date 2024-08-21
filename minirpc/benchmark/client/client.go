package main

import (
	"context"
	"fmt"
	"gamerouter/minirpc"
	echo "gamerouter/minirpc/benchmark/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
	"runtime/pprof"
	"time"
)

var (
	endpoints = []string{"127.0.0.1:2379"}
)

func main() {
	f, _ := os.Create("cpuhash.pprof")
	mem, _ := os.Create("memhash.pprof")
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()
	defer pprof.WriteHeapProfile(mem)
	cc, _ := minirpc.DialContext(context.Background(),
		"etcd://EchoServer",
		minirpc.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
		minirpc.WithEtcdHosts(endpoints),
		minirpc.WithLoadBalancer(minirpc.KetamaWeightName))
	time.Sleep(2 * time.Second)
	cli := echo.NewEchoServerClient(cc)
	for range 30 {
		ctx := minirpc.RequestScopeHashKey(context.Background(), "123")
		resp, err := cli.Echo(ctx,
			&echo.EchoRequest{Msg: "hello"})
		if err != nil {
			panic(err)
		}
		fmt.Printf("receive respond: %s \n", resp.Msg)
		time.Sleep(time.Second)
	}
}
