package clouddkcp

import (
	"fmt"
	"log"
)

const (
	rtCloud         = "CLOUD"
	rtInstances     = "INSTANCES"
	rtLoadBalancers = "LOADBALANCERS"
	rtZones         = "ZONES"
)

// debugCloudAction writes a debug message to the log.
func debugCloudAction(resourceType string, format string, v ...interface{}) {
	log.Printf(fmt.Sprintf("[%s] ", resourceType)+format, v...)
}
