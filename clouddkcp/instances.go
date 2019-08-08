package clouddkcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"

	"github.com/danitso/terraform-provider-clouddk/clouddk"
	"k8s.io/apimachinery/pkg/types"
)

// Instances implements the interface cloudprovider.Instances.
type Instances struct {
	config *CloudConfiguration
}

// newInstances initializes a new Instances object.
func newInstances(c *CloudConfiguration) cloudprovider.Instances {
	return Instances{
		config: c,
	}
}

// NodeAddresses returns the addresses of the specified instance.
func (i Instances) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	debugCloudAction(rtInstances, "Retrieving node addresses (name: %s)", string(name))

	nodeAddresses := make([]v1.NodeAddress, 0)

	server := CloudServer{
		CloudConfiguration: i.config,
	}

	_, err := server.InitializeByHostname(string(name))

	if err != nil {
		return nodeAddresses, err
	}

	for _, n := range server.Information.NetworkInterfaces {
		for _, i := range n.IPAddresses {
			nodeAddresses = append(nodeAddresses, v1.NodeAddress{
				Type:    "ExternalIP",
				Address: i.Address,
			}, v1.NodeAddress{
				Type:    "InternalIP",
				Address: i.Address,
			})
		}
	}

	return nodeAddresses, nil
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node.
// The ProviderID is a unique identifier of the node.
// This will not be called from the node whose nodeaddresses are being queried. i.e. local metadata services cannot be used in this method to obtain nodeaddresses.
func (i Instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	debugCloudAction(rtInstances, "Retrieving node addresses (id: %s)", providerID)

	nodeAddresses := make([]v1.NodeAddress, 0)

	server := CloudServer{
		CloudConfiguration: i.config,
	}

	_, err := server.InitializeByID(providerID)

	if err != nil {
		return nodeAddresses, err
	}

	for _, n := range server.Information.NetworkInterfaces {
		for _, i := range n.IPAddresses {
			nodeAddresses = append(nodeAddresses, v1.NodeAddress{
				Type:    "ExternalIP",
				Address: i.Address,
			}, v1.NodeAddress{
				Type:    "InternalIP",
				Address: i.Address,
			})
		}
	}

	return nodeAddresses, nil
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist, we must return ("", cloudprovider.InstanceNotFound).
// cloudprovider.InstanceNotFound should NOT be returned for instances that exist but are stopped/sleeping.
func (i Instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	debugCloudAction(rtInstances, "Retrieving instance id for node (name: %s)", string(nodeName))

	server := CloudServer{
		CloudConfiguration: i.config,
	}

	notFound, err := server.InitializeByHostname(string(nodeName))

	if err != nil {
		if notFound {
			return "", cloudprovider.InstanceNotFound
		}

		return "", err
	}

	debugCloudAction(rtInstances, "Node '%s' has provider id '%s'", string(nodeName), server.Information.Identifier)

	return server.Information.Identifier, nil
}

// InstanceType returns the type of the specified instance.
func (i Instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	debugCloudAction(rtInstances, "Retrieving instance type for node (name: %s)", string(name))

	server := CloudServer{
		CloudConfiguration: i.config,
	}

	_, err := server.InitializeByHostname(string(name))

	return server.Information.Package.Identifier, err
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i Instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	debugCloudAction(rtInstances, "Retrieving instance type for node (id: %s)", providerID)

	server := CloudServer{
		CloudConfiguration: i.config,
	}

	_, err := server.InitializeByID(providerID)

	return server.Information.Package.Identifier, err
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances expected format for the key is standard ssh-keygen format: <protocol> <blob>.
func (i Instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	debugCloudAction(rtInstances, "Adding SSH key for user '%s' to all node instances", user)

	return errors.New("Not implemented")
}

// CurrentNodeName returns the name of the node we are currently running on.
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname.
func (i Instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	debugCloudAction(rtInstances, "Retrieving node name for current node (name: %s)", hostname)

	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (i Instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	debugCloudAction(rtInstances, "Determining if node instance exists (id: %s)", providerID)

	server := CloudServer{
		CloudConfiguration: i.config,
	}

	notFound, err := server.InitializeByID(providerID)
	exists := err == nil || notFound == false

	if exists {
		debugCloudAction(rtInstances, "Node instance exists (id: %s)", providerID)
	} else {
		debugCloudAction(rtInstances, "Node instance does not exist (id: %s)", providerID)
	}

	return exists, err
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider.
func (i Instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	debugCloudAction(rtInstances, "Determining if node instance is powered off (id: %s)", providerID)

	server := CloudServer{
		CloudConfiguration: i.config,
	}

	_, err := server.InitializeByID(providerID)

	if err != nil {
		debugCloudAction(rtInstances, "Node instance is not powered off (id: %s)", providerID)

		return false, err
	}

	res, err := clouddk.DoClientRequest(
		i.config.ClientSettings,
		"GET",
		fmt.Sprintf("cloudservers/%s/logs", server.Information.Identifier),
		new(bytes.Buffer),
		[]int{200},
		1,
		1,
	)

	if err != nil {
		debugCloudAction(rtInstances, "Node instance is not powered off (id: %s)", providerID)

		return false, err
	}

	logsList := clouddk.LogsListBody{}
	err = json.NewDecoder(res.Body).Decode(&logsList)

	if err != nil {
		debugCloudAction(rtInstances, "Node instance is not powered off (id: %s)", providerID)

		return false, err
	}

	for _, v := range logsList {
		if v.Status == "pending" || v.Status == "running" {
			return false, nil
		}
	}

	poweredOff := (server.Information.Booted == false)

	if poweredOff {
		debugCloudAction(rtInstances, "Node instance is powered off (id: %s)", providerID)
	} else {
		debugCloudAction(rtInstances, "Node instance is not powered off (id: %s)", providerID)
	}

	return poweredOff, nil
}
