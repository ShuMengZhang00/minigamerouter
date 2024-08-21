package minirpc

import (
	"encoding/json"
	"fmt"
	"gamerouter/discover"
	"google.golang.org/grpc"
	"net"
	"strconv"
	"strings"
)

type (
	RegisterFn func(server *grpc.Server)

	ServerInfo struct {
		Namespace      string
		ServiceName    string
		InstanceID     string
		Weight         int
		Host           string
		Port           int
		ServerMetadata map[string]string
	}

	Server struct {
		*grpc.Server
		endpoints []string
		info      ServerInfo
		publisher *discover.Publisher
	}
)

type ServerOption func(option *Server)

func WithServiceName(name string) ServerOption {
	return func(s *Server) {
		s.info.ServiceName = name
	}
}

func WithInstanceID(id string) ServerOption {
	return func(s *Server) {
		s.info.InstanceID = id
	}
}

func WithServerNamespace(name string) ServerOption {
	return func(s *Server) {
		s.info.Namespace = name
	}
}

func WithServerMetadata(metadata map[string]string) ServerOption {
	return func(s *Server) {
		s.info.ServerMetadata = metadata
	}
}

func WithEtcdEndPoints(endpoints []string) ServerOption {
	return func(s *Server) {
		s.endpoints = endpoints
	}
}

func WithWeight(weight int) ServerOption {
	return func(s *Server) {
		s.info.Weight = weight
	}
}

func NewServer(info ServerInfo, options ...grpc.ServerOption) (*Server, error) {
	if len(info.Host) == 0 {
		return nil, fmt.Errorf("localhost should not be empty")
	}
	srv := &Server{
		Server: grpc.NewServer(options...),
		info:   info,
	}
	srv.initResource()
	return srv, nil
}

func (s *Server) initResource(opts ...ServerOption) {
	for _, o := range opts {
		o(s)
	}
	if len(s.info.Namespace) == 0 {
		s.info.Namespace = DefaultNamespace
	}
	if s.info.Weight == 0 {
		s.info.Weight = 1
	}
}

func (s *Server) Serve(lis net.Listener) error {
	if err := s.doRegister(lis); err != nil {
		return err
	}
	return s.Server.Serve(lis)
}

func Serve(gSrv *grpc.Server, lis net.Listener, opts ...ServerOption) error {
	_, err := Register(gSrv, lis, opts...)
	if err != nil {
		return err
	}
	return gSrv.Serve(lis)
}

func Register(gSrv *grpc.Server, lis net.Listener, opts ...ServerOption) (*Server, error) {
	srv := &Server{Server: gSrv}
	srv.initResource(opts...)
	return srv, srv.doRegister(lis)
}

func (s *Server) doRegister(lis net.Listener) error {
	if len(s.info.Host) == 0 {
		s.info.Host = strings.Split(lis.Addr().String(), ":")[0]
	}
	if s.info.Port == 0 {
		port, err := parsePort(lis.Addr().String())
		if err != nil {
			return err
		}
		s.info.Port = port
	}
	if len(s.info.InstanceID) == 0 {
		s.info.InstanceID = fmt.Sprintf("%s:%d", s.info.Host, s.info.Port)
	}
	if err := s.pubToEtcd(s.endpoints, s.info); err != nil {
		return err
	}
	return nil
}

func parsePort(addr string) (int, error) {
	colonIdx := strings.LastIndex(addr, ":")
	if colonIdx < 0 {
		return 0, fmt.Errorf("invalid addr string: %s", addr)
	}
	portStr := addr[colonIdx+1:]
	return strconv.Atoi(portStr)
}

func (s *Server) pubToEtcd(endpoints []string, conf ServerInfo) error {
	key := MakeEtcdInstanceKey(conf.Namespace, conf.ServiceName,
		conf.InstanceID)
	val, err := json.Marshal(conf)

	if err != nil {
		return err
	}
	s.publisher = discover.NewPublisher(endpoints, key, string(val))
	return s.publisher.KeepAlive()
}
