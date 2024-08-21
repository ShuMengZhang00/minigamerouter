package minirpc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"gamerouter/discover"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/resolver"
	"strings"
)

type (
	ClientConf struct {
		Etcd discover.EtcdConf
	}

	Client interface {
		Conn() *grpc.ClientConn
	}
)

const (
	lbConfig = `
{
    "loadBalancingConfig":[
        {
            "%s":{}
        }
    ]
}`
	optionsKey = "options"
)

func init() {
	resolver.Register(&etcdResolverBuilder{})
	balancer.Register(&balancerBuilder{})
}

func DialContext(ctx context.Context, target string, opts ...DialOption) (conn *grpc.ClientConn, err error) {
	options := &dialOptions{}
	for _, opt := range opts {
		opt(options)
	}

	if !strings.HasPrefix(target, EtcdScheme) {
		return grpc.DialContext(ctx, target, options.gRPCDialOptions...)
	}
	if len(options.Namespace) == 0 {
		options.Namespace = DefaultNamespace
	}

	lbStr := fmt.Sprintf(lbConfig, EtcdScheme)
	options.gRPCDialOptions = append(options.gRPCDialOptions,
		grpc.WithDefaultServiceConfig(lbStr))
	jsonConfig, err := json.Marshal(options)
	endpoint := base64.URLEncoding.EncodeToString(jsonConfig)
	target = fmt.Sprintf("%s?%s=%s", target, optionsKey, endpoint)
	return grpc.DialContext(ctx, target, options.gRPCDialOptions...)
}
