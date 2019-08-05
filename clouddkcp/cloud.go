package clouddkcp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"

	cloudprovider "k8s.io/cloud-provider"

	"github.com/danitso/terraform-provider-clouddk/clouddk"
)

const (
	// ProviderName specifies the name of the cloud controller manager defined in this file.
	ProviderName = "clouddk"

	// envAPIEndpoint specifies the name of the environment variable containing the Cloud.dk API endpoint.
	envAPIEndpoint = "CLOUDDK_API_ENDPOINT"

	// envAPIKey specifies the name of the environment variable containing the Cloud.dk API key.
	envAPIKey = "CLOUDDK_API_KEY"

	// envSSHPrivateKey specifies the name of the environment variable containing the Base 64 encoded private key for SSH connections.
	envSSHPrivateKey = "CLOUDDK_SSH_PRIVATE_KEY"

	// envSSHPublicKey specifies the name of the environment variable containing the Base 64 encoded public key for SSH connections.
	envSSHPublicKey = "CLOUDDK_SSH_PUBLIC_KEY"
)

// Cloud implements the interface cloudprovider.Interface.
type Cloud struct {
	loadBalancers cloudprovider.LoadBalancer
	instances     cloudprovider.Instances
	zones         cloudprovider.Zones
}

// CloudConfiguration stores the cloud configuration.
type CloudConfiguration struct {
	ClientSettings *clouddk.ClientSettings
	PrivateKey     string
	PublicKey      string
}

// init registers this cloud provider.
func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(io.Reader) (cloudprovider.Interface, error) {
		return newCloud()
	})
}

// newCloud initializes a new Cloud object.
func newCloud() (cloudprovider.Interface, error) {
	config := CloudConfiguration{}
	config.ClientSettings.Endpoint = os.Getenv(envAPIEndpoint)

	if config.ClientSettings.Endpoint == "" {
		config.ClientSettings.Endpoint = "https://api.cloud.dk/v1"
	}

	config.ClientSettings.Key = os.Getenv(envAPIKey)

	if config.ClientSettings.Key == "" {
		return nil, fmt.Errorf("The environment variable '%s' is empty", envAPIKey)
	}

	config.PrivateKey = os.Getenv(envSSHPrivateKey)

	if config.PrivateKey != "" {
		key, err := base64.StdEncoding.DecodeString(config.PrivateKey)

		if err != nil {
			return nil, err
		}

		config.PrivateKey = bytes.NewBuffer(key).String()
	} else {
		return nil, fmt.Errorf("The environment variable '%s' is empty", envSSHPrivateKey)
	}

	config.PublicKey = os.Getenv(envSSHPublicKey)

	if config.PublicKey != "" {
		key, err := base64.StdEncoding.DecodeString(config.PublicKey)

		if err != nil {
			return nil, err
		}

		config.PublicKey = bytes.NewBuffer(key).String()
	} else {
		return nil, fmt.Errorf("The environment variable '%s' is empty", envSSHPublicKey)
	}

	return Cloud{
		loadBalancers: newLoadBalancers(&config),
		instances:     newInstances(&config),
		zones:         newZones(&config),
	}, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (c Cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	fmt.Printf("Initializing cloud provider '%s'\n", ProviderName)
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (c Cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadBalancers, false
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

// HasClusterID returns true if a ClusterID is required and set.
func (c Cloud) HasClusterID() bool {
	return false
}
