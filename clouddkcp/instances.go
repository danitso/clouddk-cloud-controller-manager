package clouddkcp

import (
	"context"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"

	"k8s.io/apimachinery/pkg/types"
)

type Instances struct{}

// newInstances initializes a new Instances object
func newInstances() cloudprovider.Instances {
	return Instances{}
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

	return nodeAddresses, nil
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist, we must return ("", cloudprovider.InstanceNotFound)
// cloudprovider.InstanceNotFound should NOT be returned for instances that exist but are stopped/sleeping
func (i Instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	return "", nil
}

// InstanceType returns the type of the specified instance.
func (i Instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	return "", nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i Instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	return "", nil
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i Instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return nil
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i Instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return "", nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (i Instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	return false, nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i Instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	return false, nil
}
