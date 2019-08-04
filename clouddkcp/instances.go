package clouddkcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"

	"github.com/danitso/terraform-provider-clouddk/clouddk"
	"k8s.io/apimachinery/pkg/types"
)

type Instances struct {
	ClientSettings *clouddk.ClientSettings
}

// newInstances initializes a new Instances object
func newInstances(cs *clouddk.ClientSettings) cloudprovider.Instances {
	return Instances{
		ClientSettings: cs,
	}
}

// GetServerObjectByID retrieves the ServerBody object for a specific node.
func (i Instances) GetServerObjectByID(providerID string) (clouddk.ServerBody, error) {
	res, resErr := clouddk.DoClientRequest(i.ClientSettings, "GET", fmt.Sprintf("cloudservers/%s", providerID), new(bytes.Buffer), []int{200}, 1, 1)

	if resErr != nil {
		return clouddk.ServerBody{}, resErr
	}

	server := clouddk.ServerBody{}
	decodeErr := json.NewDecoder(res.Body).Decode(&server)

	if decodeErr != nil {
		return clouddk.ServerBody{}, decodeErr
	}

	return server, nil
}

// GetServerObjectByNodeName retrieves the ServerBody object for a specific node.
func (i Instances) GetServerObjectByNodeName(nodeName types.NodeName) (clouddk.ServerBody, error) {
	res, resErr := clouddk.DoClientRequest(i.ClientSettings, "GET", fmt.Sprintf("cloudservers?hostname=%s", nodeName), new(bytes.Buffer), []int{200}, 1, 1)

	if resErr != nil {
		return clouddk.ServerBody{}, resErr
	}

	servers := make(clouddk.ServerListBody, 0)
	decodeErr := json.NewDecoder(res.Body).Decode(&servers)

	if decodeErr != nil {
		return clouddk.ServerBody{}, decodeErr
	}

	for _, v := range servers {
		if v.Hostname == string(nodeName) {
			return v, nil
		}
	}

	return clouddk.ServerBody{}, fmt.Errorf("Failed to retrieve the server object for noode '%s'", nodeName)
}

// NodeAddresses returns the addresses of the specified instance.
func (i Instances) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	nodeAddresses := make([]v1.NodeAddress, 0)

	return nodeAddresses, nil
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node.
// The ProviderID is a unique identifier of the node.
// This will not be called from the node whose nodeaddresses are being queried. i.e. local metadata services cannot be used in this method to obtain nodeaddresses
func (i Instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	nodeAddresses := make([]v1.NodeAddress, 0)
	server, serverErr := i.GetServerObjectByID(providerID)

	if serverErr != nil {
		return nodeAddresses, serverErr
	}

	for _, n := range server.NetworkInterfaces {
		for _, i := range n.IPAddresses {
			nodeAddresses = append(nodeAddresses, v1.NodeAddress{
				Type:    "ExternalIP",
				Address: i.Address,
			})
		}
	}

	return nodeAddresses, nil
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist, we must return ("", cloudprovider.InstanceNotFound)
// cloudprovider.InstanceNotFound should NOT be returned for instances that exist but are stopped/sleeping
func (i Instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	server, serverErr := i.GetServerObjectByNodeName(nodeName)

	return server.Identifier, serverErr
}

// InstanceType returns the type of the specified instance.
func (i Instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	server, serverErr := i.GetServerObjectByNodeName(name)

	return server.Package.Identifier, serverErr
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i Instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	server, serverErr := i.GetServerObjectByID(providerID)

	return server.Package.Identifier, serverErr
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i Instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return errors.New("Not implemented")
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i Instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	hostname, hostnameErr := os.Hostname()

	return types.NodeName(hostname), hostnameErr
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (i Instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	_, serverErr := i.GetServerObjectByID(providerID)

	return (serverErr == nil), nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i Instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	server, serverErr := i.GetServerObjectByID(providerID)

	if serverErr != nil {
		return false, serverErr
	}

	res, resErr := clouddk.DoClientRequest(i.ClientSettings, "GET", fmt.Sprintf("cloudservers/%s/logs", server.Identifier), new(bytes.Buffer), []int{200}, 1, 1)

	if resErr != nil {
		return false, resErr
	}

	logsList := clouddk.LogsListBody{}
	json.NewDecoder(res.Body).Decode(&logsList)

	for _, v := range logsList {
		if v.Status == "pending" || v.Status == "running" {
			return false, nil
		}
	}

	return (server.Booted == false), nil
}
