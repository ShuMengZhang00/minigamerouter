package minirpc

import "gamerouter/discover"

type (
	ServerConf struct {
		Etcd       discover.EtcdConf
		ServerName string
		// optional fields
		InstanceId string
	}
)
