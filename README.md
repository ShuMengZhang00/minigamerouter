discovery/


封装了和etcd的交互，publisher，subscriber提供注册订阅等功能


minirpc/


一个基于gRPC的库，提供了基于etcd的服务注册/发现，负载均衡等功能，对应的压测脚本和example位于minirpc/benchmark


router/


一个使用minirpc的rpc server，提供动态键值路由规则的设置和查询功能，测试的路由表和规则生成程序位于router/client，基于ghz的压测程序位于router/prefix_bench
