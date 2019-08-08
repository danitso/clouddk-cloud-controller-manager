package clouddkcp

import (
	"fmt"
	"log"
	"strings"
)

const (
	rtCloud         = "CLOUD"
	rtInstances     = "INSTANCES"
	rtLoadBalancers = "LOADBALANCERS"
	rtServers       = "SERVERS"
	rtZones         = "ZONES"
)

// debugCloudAction writes a debug message to the log.
func debugCloudAction(resourceType string, format string, v ...interface{}) {
	log.Printf(fmt.Sprintf("[%s] ", resourceType)+format, v...)
}

// trimProviderID removes the provider name from the id.
func trimProviderID(id string) string {
	return strings.TrimPrefix(id, "clouddk://")
}
