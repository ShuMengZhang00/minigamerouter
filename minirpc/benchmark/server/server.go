package main

import (
	"context"
	"flag"
	"fmt"
	"gamerouter/minirpc"
	echo "gamerouter/minirpc/benchmark/proto"
	"google.golang.org/grpc"
	"log"
	"net"
	"sync"
	"time"
)

var (
	RouterServiceName = "EchoServer"
	endpoints         = []string{"127.0.0.1:2379"}
	addrPattern       = "localhost:%d"
	startPort         = flag.Int("startPort", 60000, "server start port")
	num               = flag.Int("num", 10, "server number")
)

type Server struct {
	listen net.Listener

	echo.UnimplementedEchoServerServer
}

func (s Server) Echo(ctx context.Context, request *echo.EchoRequest) (*echo.EchoReply, error) {
	msg := fmt.Sprintf("Request msg: %s, my addr is %s", request.Msg,
		s.listen.Addr().String())
	return &echo.EchoReply{Msg: msg}, nil
}

func startAServer(port int, wg *sync.WaitGroup) {
	addr := fmt.Sprintf(addrPattern, port)
	listen, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to addr %s: %v", addr, err)
	}
	fmt.Printf("router is listening at %s\n", listen.Addr().String())
	srv := grpc.NewServer()
	echo.RegisterEchoServerServer(srv, Server{listen: listen})

	if err = minirpc.Serve(srv, listen,
		minirpc.WithServerNamespace(minirpc.DefaultNamespace),
		minirpc.WithServiceName(RouterServiceName),
		minirpc.WithEtcdEndPoints(endpoints)); err != nil {
		log.Printf("lisen err: %v", err)
	}
	wg.Done()
}

func main() {
	flag.Parse()
	offset := *startPort
	var wg sync.WaitGroup
	wg.Add(*num)
	for i := range *num {
		go startAServer(offset+i, &wg)
		time.Sleep(time.Millisecond)
	}
	wg.Wait()
}
