package clouddkcp

import (
	"fmt"
	"log"
)

const (
	rtCloud         = "Cloud"
	rtInstances     = "Instances"
	rtLoadBalancers = "LoadBalancers"
	rtZones         = "Zones"
)

// debugCloudAction writes a debug message to the log.
func debugCloudAction(resourceType string, format string, v ...interface{}) {
	log.Printf(fmt.Sprintf("[%s] ", resourceType)+format, v...)
}
