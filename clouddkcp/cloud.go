package clouddkcp

import (
	"fmt"
	"io"

	cloudprovider "k8s.io/cloud-provider"

	"github.com/danitso/terraform-provider-clouddk/clouddk"
)

const (
	ProviderName = "clouddk"
)

// Cloud implements the interface cloudprovider.Interface
type Cloud struct {
	clientSettings clouddk.ClientSettings
	loadBalancers  cloudprovider.LoadBalancer
	instances      cloudprovider.Instances
	zones          cloudprovider.Zones
}

// init registers this cloud provider
func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(io.Reader) (cloudprovider.Interface, error) {
		return newCloud()
	})
}

// newCloud initializes a new Cloud object
func newCloud() (cloudprovider.Interface, error) {
	return Cloud{
		loadBalancers: newLoadBalancers(),
		instances:     newInstances(),
		zones:         newZones(),
	}, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (c Cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	fmt.Printf("Initializing cloud provider '%s'\n", ProviderName)
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (c Cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadBalancers, true
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (c Cloud) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (c Cloud) Zones() (cloudprovider.Zones, bool) {
	return c.zones, true
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (c Cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (c Cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c Cloud) ProviderName() string {
	return ProviderName
}

// HasClusterID returns true if a ClusterID is required and set
func (c Cloud) HasClusterID() bool {
	return false
}
