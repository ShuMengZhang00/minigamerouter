package main

import (
	"gamerouter/minirpc"
	router "gamerouter/router/proto"
	"github.com/bojand/ghz/printer"
	"github.com/bojand/ghz/runner"
	"github.com/gogo/protobuf/proto"
	"log"
	"os"
)

func main() {
	item := router.GetEndpointWithPrefixRequest{
		ServiceName: "testserver",
		Namespace:   minirpc.DefaultNamespace,
		Key:         "random123",
	}
	buf := proto.Buffer{}
	err := buf.EncodeMessage(&item)
	report, err := runner.Run(
		"router.Router.GetOneInstanceWithPrefix",
		"127.0.0.1:50051",
		runner.WithProtoFile("../proto/router.proto", []string{}),
		runner.WithBinaryData(buf.Bytes()),
		runner.WithInsecure(true),
		runner.WithTotalRequests(100000),
		runner.WithConcurrency(100),
		runner.WithSkipTLSVerify(true),
	)

	if err != nil {
		log.Fatal(err)
		return
	}
	file, err := os.Create("report100c.html")
	rp := printer.ReportPrinter{
		Out:    file,
		Report: report,
	}
	_ = rp.Print("html")
}
