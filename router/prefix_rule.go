package route

import (
	"gamerouter/discover"
	"sync"
)

var (
	RouteRulePrefix = "/rule"
)

type RuleTable struct {
	serviceToPrefix map[string]*RadixTree // service prefix rule
	mu              sync.RWMutex
}

func NewRuleTable(endpoint []string) *RuleTable {
	r := &RuleTable{
		serviceToPrefix: make(map[string]*RadixTree),
	}
	discover.GetRegistry(endpoint).Monitor(RouteRulePrefix+"/", r)
	return r
}

func (r *RuleTable) OnAdd(kv discover.KV) {
	namespace, servicename, prefix := extractEtcdKey(kv.Key)
	instanceID := kv.Val
	r.updateRule(namespace, servicename, prefix, instanceID)
}

func (r *RuleTable) OnDelete(kv discover.KV) {
	namespace, servicename, prefix := extractEtcdKey(kv.Key)
	r.removeRule(namespace, servicename, prefix)
}

func getServiceKey(namespace, serviceName string) string {
	return namespace + "/" + serviceName
}

func (r *RuleTable) GetInstanceID(namespace, serviceName, routeKey string) (string, bool) {
	r.mu.RLock()
	key := getServiceKey(namespace, serviceName)
	tree, ok := r.serviceToPrefix[key]
	r.mu.RUnlock()
	if !ok || tree == nil {
		return "", false
	}

	_, instanceID, ok := tree.LongestPrefix(routeKey)
	if !ok {
		return "", false
	}
	return instanceID.(string), ok
}

func (r *RuleTable) updateRule(namespace, serviceName, prefix, instanceID string) {
	key := getServiceKey(namespace, serviceName)
	r.mu.Lock()
	tree, ok := r.serviceToPrefix[key]
	if !ok || tree == nil {
		r.serviceToPrefix[key] = NewRadixTree()
	}
	r.serviceToPrefix[key].Insert(prefix, instanceID)
	r.mu.Unlock()
}

func (r *RuleTable) removeRule(namespace, serviceName, prefix string) {
	key := getServiceKey(namespace, serviceName)
	r.mu.Lock()
	tree, ok := r.serviceToPrefix[key]
	r.mu.Unlock()
	if !ok || tree == nil {
		return
	}
	tree.Delete(prefix)
}
