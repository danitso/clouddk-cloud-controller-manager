package clouddkcp

import (
	"context"
	"os"

	cloudprovider "k8s.io/cloud-provider"

	"k8s.io/apimachinery/pkg/types"
)

// Zones implements the interface cloudprovider.Zones.
type Zones struct {
	config *CloudConfiguration
}

// newZones initializes a new Zones object.
func newZones(c *CloudConfiguration) cloudprovider.Zones {
	return Zones{
		config: c,
	}
}

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in.
// In most cases, this method is called from the kubelet querying a local metadata service to acquire its zone.
// For the case of external cloud providers, use GetZoneByProviderID or GetZoneByNodeName since GetZone can no longer be called from the kubelets.
func (z Zones) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	hostname, err := os.Hostname()

	debugCloudAction(rtLoadBalancers, "Retrieving zone for node instance (name: %s)", hostname)

	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return z.GetZoneByNodeName(ctx, types.NodeName(hostname))
}

// GetZoneByProviderID returns the Zone containing the current zone and locality region of the node specified by providerID.
// This method is particularly used in the context of external cloud providers where node initialization must be done outside the kubelets.
func (z Zones) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	debugCloudAction(rtLoadBalancers, "Retrieving zone for node instance (id: %s)", providerID)

	zone := cloudprovider.Zone{}

	server := CloudServer{
		CloudConfiguration: z.config,
	}

	_, err := server.InitializeByID(providerID)

	if err != nil {
		return zone, err
	}

	zone.FailureDomain = server.Information.Location.Identifier
	zone.Region = server.Information.Location.Identifier

	return zone, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node name.
// This method is particularly used in the context of external cloud providers where node initialization must be done outside the kubelets.
func (z Zones) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	debugCloudAction(rtLoadBalancers, "Retrieving zone for node instance (name: %s)", string(nodeName))

	zone := cloudprovider.Zone{}

	server := CloudServer{
		CloudConfiguration: z.config,
	}

	_, err := server.InitializeByHostname(string(nodeName))

	if err != nil {
		return zone, err
	}

	zone.FailureDomain = server.Information.Location.Identifier
	zone.Region = server.Information.Location.Identifier

	return zone, nil
}
