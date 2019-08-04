package clouddkcp

import (
	"context"
	"os"

	cloudprovider "k8s.io/cloud-provider"

	"github.com/danitso/terraform-provider-clouddk/clouddk"
	"k8s.io/apimachinery/pkg/types"
)

type Zones struct {
	ClientSettings *clouddk.ClientSettings
}

// newZones initializes a new Zones object
func newZones(cs *clouddk.ClientSettings) cloudprovider.Zones {
	return Zones{
		ClientSettings: cs,
	}
}

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in
// In most cases, this method is called from the kubelet querying a local metadata service to acquire its zone.
// For the case of external cloud providers, use GetZoneByProviderID or GetZoneByNodeName since GetZone
// can no longer be called from the kubelets.
func (z Zones) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	hostname, hostnameErr := os.Hostname()

	if hostnameErr != nil {
		return cloudprovider.Zone{}, hostnameErr
	}

	return z.GetZoneByNodeName(ctx, types.NodeName(hostname))
}

// GetZoneByProviderID returns the Zone containing the current zone and locality region of the node specified by providerID
// This method is particularly used in the context of external cloud providers where node initialization must be done
// outside the kubelets.
func (z Zones) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	zone := cloudprovider.Zone{}
	server, serverErr := GetServerObjectByID(z.ClientSettings, providerID)

	if serverErr != nil {
		return zone, serverErr
	}

	zone.FailureDomain = server.Location.Identifier
	zone.Region = server.Location.Identifier

	return zone, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node name
// This method is particularly used in the context of external cloud providers where node initialization must be done
// outside the kubelets.
func (z Zones) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	zone := cloudprovider.Zone{}
	server, serverErr := GetServerObjectByNodeName(z.ClientSettings, nodeName)

	if serverErr != nil {
		return zone, serverErr
	}

	zone.FailureDomain = server.Location.Identifier
	zone.Region = server.Location.Identifier

	return zone, nil
}
