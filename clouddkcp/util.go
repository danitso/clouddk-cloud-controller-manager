package clouddkcp

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/danitso/terraform-provider-clouddk/clouddk"
	"k8s.io/apimachinery/pkg/types"
)

// GetServerObjectByID retrieves the ServerBody object for a specific node.
func GetServerObjectByID(cs *clouddk.ClientSettings, providerID string) (clouddk.ServerBody, error) {
	res, resErr := clouddk.DoClientRequest(cs, "GET", fmt.Sprintf("cloudservers/%s", providerID), new(bytes.Buffer), []int{200}, 1, 1)

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
func GetServerObjectByNodeName(cs *clouddk.ClientSettings, nodeName types.NodeName) (clouddk.ServerBody, error) {
	res, resErr := clouddk.DoClientRequest(cs, "GET", fmt.Sprintf("cloudservers?hostname=%s", nodeName), new(bytes.Buffer), []int{200}, 1, 1)

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
