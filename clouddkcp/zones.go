package clouddkcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	res, resErr := clouddk.DoClientRequest(z.ClientSettings, "GET", fmt.Sprintf("cloudservers/%s", providerID), new(bytes.Buffer), []int{200}, 1, 1)

	if resErr != nil {
		return zone, resErr
	}

	server := clouddk.ServerBody{}
	decodeErr := json.NewDecoder(res.Body).Decode(&server)

	if decodeErr != nil {
		return zone, decodeErr
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
	res, resErr := clouddk.DoClientRequest(z.ClientSettings, "GET", fmt.Sprintf("cloudservers?hostname=%s", nodeName), new(bytes.Buffer), []int{200}, 1, 1)

	if resErr != nil {
		return zone, resErr
	}

	servers := make(clouddk.ServerListBody, 0)
	decodeErr := json.NewDecoder(res.Body).Decode(&servers)

	if decodeErr != nil {
		return zone, decodeErr
	}

	for _, v := range servers {
		if v.Hostname == string(nodeName) {
			zone.FailureDomain = v.Location.Identifier
			zone.Region = v.Location.Identifier

			break
		}
	}

	if zone.Region == "" {
		return zone, fmt.Errorf("Failed to determine the zone for node '%s'", nodeName)
	}

	return zone, nil
}
