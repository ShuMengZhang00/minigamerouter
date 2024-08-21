package minirpc

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	miniRequestLbHashKey = "mini.request.hashKey"
	miniRequestLbPolicy  = "mini.request.lbPolicy"
)

func RequestScopeHashKey(ctx context.Context, key string) context.Context {
	_, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{
			miniRequestLbHashKey: key,
		}))

		return ctx
	}

	return metadata.AppendToOutgoingContext(ctx, miniRequestLbHashKey, key)
}

func RequestScopeLbPolicy(ctx context.Context, policy string) context.Context {
	_, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		ctx = metadata.NewOutgoingContext(ctx, metadata.New(map[string]string{
			miniRequestLbPolicy: policy,
		}))

		return ctx
	}

	return metadata.AppendToOutgoingContext(ctx, miniRequestLbPolicy, policy)
}

type DialOption func(options *dialOptions)

type dialOptions struct {
	gRPCDialOptions []grpc.DialOption
	Endpoints       []string
	Namespace       string
	LbPolicy        string
	DstMetadata     map[string]string
	RouteKey        string
	HashKey         string
}

func WithGRPCDialOptions(opts ...grpc.DialOption) DialOption {
	return func(options *dialOptions) {
		options.gRPCDialOptions = opts
	}
}

func WithNamespace(namespace string) DialOption {
	return func(options *dialOptions) {
		options.Namespace = namespace
	}
}

func WithEtcdHosts(endpoints []string) DialOption {
	return func(options *dialOptions) {
		options.Endpoints = endpoints
	}
}

func WithLoadBalancer(lb string) DialOption {
	return func(options *dialOptions) {
		options.LbPolicy = lb
	}
}

func WithDstMetadata(metadata map[string]string) DialOption {
	return func(options *dialOptions) {
		options.DstMetadata = metadata
	}
}

func WithRouteKey(routeKey string) DialOption {
	return func(options *dialOptions) {
		options.RouteKey = routeKey
	}
}
