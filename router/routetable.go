package route

import (
	"encoding/json"
	"gamerouter/discover"
	"gamerouter/minirpc"
	"log"
	"strings"
	"sync"
)

type RouteTable struct {
	table sync.Map // namespace/servicename/instanceID->serverInfo
}

func NewRouteTable(endpoint []string) *RouteTable {
	r := &RouteTable{}
	discover.GetRegistry(endpoint).Monitor(minirpc.RouteIp+"/", r)
	return r
}

func extractEtcdKey(key string) (namespace, serviceName, instanceID string) {
	split := strings.Split(key, "/")
	namespace = split[2]
	serviceName = split[3]
	instanceID = split[4]
	return
}

func (r *RouteTable) OnAdd(kv discover.KV) {
	namespace, serviceName, instanceID := extractEtcdKey(kv.Key)
	key := getInstanceKey(namespace, serviceName, instanceID)
	var info minirpc.ServerInfo
	if err := json.Unmarshal([]byte(kv.Val), &info); err != nil {
		log.Printf("Failed to unmarshal service info: %v", err)
		return
	}
	r.table.Store(key, info)
}

func (r *RouteTable) OnDelete(kv discover.KV) {
	namespace, serviceName, instanceID := extractEtcdKey(kv.Key)
	key := getInstanceKey(namespace, serviceName, instanceID)
	r.table.Delete(key)
}

func getInstanceKey(namespace, serviceName, instanceID string) string {
	return namespace + "/" + serviceName + "/" + instanceID
}

func (r *RouteTable) GetServerInfo(namespace, serviceName, instanceID string) (*minirpc.ServerInfo, bool) {
	key := getInstanceKey(namespace, serviceName, instanceID)
	info, ok := r.table.Load(key)
	serverinfo, _ := info.(minirpc.ServerInfo)
	return &serverinfo, ok
}
