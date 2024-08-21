package minirpc

import (
	"fmt"
)

const slashSeparator = "/"
const EndPointSepChar = ','
const RouteIp = "/routeip"

var (
	EndPointSep = fmt.Sprintf("%c", EndPointSepChar)
)

func MakeEtcdInstanceKey(namespace, servicename, instanceID string) string {
	return fmt.Sprintf("%s/%s/%s/%s", RouteIp, namespace, servicename,
		instanceID)
}

func MakeEtcdServiceKey(namespace, servicename string) string {
	return fmt.Sprintf("%s/%s/%s", RouteIp, namespace, servicename)
}
