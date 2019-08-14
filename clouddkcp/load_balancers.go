/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package clouddkcp

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"

	"github.com/pkg/sftp"
)

const (
	// annoLoadBalancerAlgorithm is the annotation specifying which load balancing algorithm to use.
	// Options are leastconn, roundrobin and source.
	// Defaults to roundrobin.
	annoLoadBalancerAlgorithm = "kubernetes.cloud.dk/load-balancer-algorithm"

	// annoLoadBalancerClientTimeout is the annotation used to specify the number of seconds the Load Balancer will allow a client to idle for.
	// The value must be between 1 and 86400.
	// Defaults to 30.
	annoLoadBalancerClientTimeout = "kubernetes.cloud.dk/load-balancer-client-timeout"

	// annoLoadBalancerConnectionLimit is the annotation specifying the connection limit.
	// The value must be between 1 and 20000.
	// Defaults to 1000.
	annoLoadBalancerConnectionLimit = "kubernetes.cloud.dk/load-balancer-connection-limit"

	// annoLoadBalancerEnableProxyProtocol is the annotation specifying whether the PROXY protocol should be enabled.
	// Defaults to false.
	annoLoadBalancerEnableProxyProtocol = "kubernetes.cloud.dk/load-balancer-enable-proxy-protocol"

	// annoLoadBalancerHealthCheckInternal is the annotation used to specify the number of seconds between between two consecutive health checks.
	// The value must be between 3 and 300.
	// Defaults to 3.
	annoLoadBalancerHealthCheckInterval = "kubernetes.cloud.dk/load-balancer-health-check-interval"

	// annoLoadBalancerHealthCheckThresholdHealthy is the annotation used to specify the number of times a health check must pass for a backend to be marked "healthy" for the given service and be re-added to the pool.
	// The value must be between 2 and 10.
	// Defaults to 5.
	annoLoadBalancerHealthCheckThresholdHealthy = "kubernetes.cloud.dk/load-balancer-health-check-threshold-healthy"

	// annoLoadBalancerHealthCheckThresholdUnhealthy is the annotation used to specify the number of times a health check must fail for a backend to be marked "unhealthy" and be removed from the pool for the given service.
	// The value must be between 2 and 10.
	// Defaults to 3.
	annoLoadBalancerHealthCheckThresholdUnhealthy = "kubernetes.cloud.dk/load-balancer-health-check-threshold-unhealthy"

	// annoLoadBalancerHealthCheckTimeout is the annotation used to specify the number of seconds the Load Balancer will wait for a response until marking a health check as failed.
	// The value must be between 3 and 300.
	// Defaults to 5.
	annoLoadBalancerHealthCheckTimeout = "kubernetes.cloud.dk/load-balancer-health-check-timeout"

	// annoLoadBalancerID is the annotation specifying the load balancer ID used to enable fast retrievals of load balancers from the API.
	annoLoadBalancerID = "kubernetes.cloud.dk/load-balancer-id"

	// annoLoadBalancerServerTimeout is the annotation used to specify the number of seconds the Load Balancer will allow a server to idle for.
	// The value must be between 1 and 86400.
	// Defaults to 60.
	annoLoadBalancerServerTimeout = "kubernetes.cloud.dk/load-balancer-server-timeout"

	// fmtLoadBalancerHostname specifies the format for load balancer hostnames.
	fmtLoadBalancerHostname = "k8s-load-balancer-%s"
)

// LoadBalancers implements the interface cloudprovider.LoadBalancer.
type LoadBalancers struct {
	config *CloudConfiguration
}

// createLoadBalancer creates a new load balancer.
func createLoadBalancer(c *CloudConfiguration, hostname string, service *v1.Service) (CloudServer, error) {
	loadBalancerName := getLoadBalancerNameByService(service)

	debugCloudAction(rtLoadBalancers, "Creating new load balancer (name: %s)", loadBalancerName)

	server := CloudServer{
		CloudConfiguration: c,
	}

	connectionLimit, err := parseIntAnnotation(service.Annotations[annoLoadBalancerConnectionLimit], 1000, 1, 20000)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to parse annotation '%s' for load balancer (name: %s)", annoLoadBalancerConnectionLimit, loadBalancerName)

		return server, err
	}

	debugCloudAction(rtLoadBalancers, "Creating cloud server for load balancer (name: %s)", loadBalancerName)

	packageID := getPackageIDByConnectionLimit(connectionLimit)
	err = server.Create("dk1", packageID, hostname)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to create cloud server for load balancer (name: %s)", loadBalancerName)

		return server, err
	}

	debugCloudAction(rtLoadBalancers, "Successfully created cloud server for load balancer (name: %s)", loadBalancerName)

	// Install an LTS version of HAProxy on the server.
	debugCloudAction(rtLoadBalancers, "Establishing SSH connection to load balancer (name: %s)", loadBalancerName)

	sshClient, err := server.SSH()

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to establish SSH connection to load balancer (name: %s)", loadBalancerName)

		return server, err
	}

	defer sshClient.Close()

	debugCloudAction(rtLoadBalancers, "Creating new SSH session for load balancer (name: %s)", loadBalancerName)

	sshSession, err := sshClient.NewSession()

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to create new SSH session for load balancer (name: %s)", loadBalancerName)

		server.Destroy()

		return server, err
	}

	defer sshSession.Close()

	debugCloudAction(rtLoadBalancers, "Provisioning cloud server for load balancer (name: %s)", loadBalancerName)

	output, err := sshSession.CombinedOutput(
		"export DEBIAN_FRONTEND=noninteractive && " +
			"while fuser /var/lib/apt/lists/lock >/dev/null 2>&1; do sleep 1; done && " +
			"while fuser /var/lib/dpkg/lock >/dev/null 2>&1; do sleep 1; done && " +
			"apt-get -qq update && " +
			"apt-get -qq install -y software-properties-common && " +
			"add-apt-repository -y ppa:vbernat/haproxy-2.0 && " +
			"apt-get -qq install -y haproxy=2.0.\\* && " +
			"mkdir -p /etc/systemd/system/haproxy.service.d && " +
			"echo '[Service]' > /etc/systemd/system/haproxy.service.d/override.conf && " +
			"echo 'LimitNOFILE=1048576' >> /etc/systemd/system/haproxy.service.d/override.conf && " +
			"chmod a+r /etc/systemd/system/haproxy.service.d/override.conf && " +
			"systemctl daemon-reload && " +
			"systemctl restart haproxy",
	)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to provision cloud server for load balancer (name: %s) - Output: %s - Error: %s", loadBalancerName, string(output), err.Error())

		server.Destroy()

		return server, err
	}

	return server, nil
}

// getLoadBalancerHostname retrieves the hostname for a load balancer.
func getLoadBalancerHostname(clusterName string, loadBalancerName string) string {
	loadBalancerHash := md5.New()

	io.WriteString(loadBalancerHash, clusterName)
	io.WriteString(loadBalancerHash, loadBalancerName)

	return fmt.Sprintf(fmtLoadBalancerHostname, fmt.Sprintf("%x", loadBalancerHash.Sum(nil)))
}

// getLoadBalancerNameByService retrieves the default load balancer name for a service.
func getLoadBalancerNameByService(service *v1.Service) string {
	name := strings.Replace(string(service.UID), "-", "", -1)

	if len(name) > 32 {
		name = name[:32]
	}

	return name
}

// getPackageIDByConnectionLimit retrieves the package id based on a connection limit.
func getPackageIDByConnectionLimit(limit int) string {
	if limit <= 1000 {
		return "89833c1dfa7010"
	} else if limit <= 10000 {
		return "e991abd8ef15c7"
	} else {
		return "9559dbb4b71c45"
	}
}

// getProcessorCountByConnectionLimit retrieves the package id based on a connection limit.
func getProcessorCountByConnectionLimit(limit int) int {
	if limit <= 1000 {
		return 1
	} else if limit <= 10000 {
		return 2
	} else {
		return 4
	}
}

// newLoadBalancers initializes a new LoadBalancers object.
func newLoadBalancers(c *CloudConfiguration) cloudprovider.LoadBalancer {
	return LoadBalancers{
		config: c,
	}
}

// parseBoolAnnotation parses an annotation containing a boolean.
func parseBoolAnnotation(value string, defaultValue bool) (bool, error) {
	if value == "" {
		return defaultValue, nil
	}

	return (value == "true"), nil
}

// parseIntAnnotation parses an annotation containing an integer.
func parseIntAnnotation(value string, defaultValue int, minValue int, maxValue int) (int, error) {
	if value == "" {
		return defaultValue, nil
	}

	i, err := strconv.Atoi(value)

	if err != nil {
		return i, err
	}

	if i < minValue {
		return i, fmt.Errorf("The value must be greater than %d (value: %d)", minValue, i)
	}

	if i > maxValue {
		return i, fmt.Errorf("The value must be smaller than %d (value: %d)", maxValue, i)
	}

	return i, nil
}

// parseStringAnnotation parses an annotation containing a string.
func parseStringAnnotation(value string, defaultValue string, supportedValues []string) (string, error) {
	if value == "" {
		return defaultValue, nil
	}

	for _, s := range supportedValues {
		if value == s {
			return s, nil
		}
	}

	return value, fmt.Errorf("Unsupported value '%s'", value)
}

// sanitizeClusterName sanitizes a cluster name for use in hostnames.
func sanitizeClusterName(clusterName string) string {
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name := re.ReplaceAllString(clusterName, "-")

	if len(name) > 32 {
		name = name[:32]
	}

	return name
}

// GetLoadBalancer returns whether the specified load balancer exists, and if so, what its status is.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l LoadBalancers) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	loadBalancerName := getLoadBalancerNameByService(service)
	hostname := getLoadBalancerHostname(clusterName, loadBalancerName)

	debugCloudAction(rtLoadBalancers, "Determining if load balancer exists (name: %s)", loadBalancerName)

	server := CloudServer{
		CloudConfiguration: l.config,
	}

	notFound, err := server.InitializeByHostname(hostname)

	if err != nil {
		if notFound {
			return &v1.LoadBalancerStatus{}, false, nil
		}

		return &v1.LoadBalancerStatus{}, true, err
	}

	ingresses := make([]v1.LoadBalancerIngress, 0)

	for _, nic := range server.Information.NetworkInterfaces {
		for _, ip := range nic.IPAddresses {
			debugCloudAction(rtLoadBalancers, "Adding IP address '%s' to load balancer ingress (name: %s)", ip.Address, loadBalancerName)

			ingresses = append(ingresses, v1.LoadBalancerIngress{
				IP: ip.Address,
			})
		}
	}

	if len(ingresses) == 0 {
		return &v1.LoadBalancerStatus{}, true, fmt.Errorf("No IP addresses available for load balancer (name: %s)", loadBalancerName)
	}

	return &v1.LoadBalancerStatus{Ingress: ingresses}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
func (l LoadBalancers) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	return getLoadBalancerNameByService(service)
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer.
// Implementations must treat the *v1.Service and *v1.Node parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l LoadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	loadBalancerName := getLoadBalancerNameByService(service)
	hostname := getLoadBalancerHostname(clusterName, loadBalancerName)

	debugCloudAction(rtLoadBalancers, "Ensuring that load balancer exists (name: %s)", loadBalancerName)

	server := CloudServer{
		CloudConfiguration: l.config,
	}

	notFound, err := server.InitializeByHostname(hostname)

	if err != nil {
		if !notFound {
			return nil, err
		}

		server, err = createLoadBalancer(l.config, hostname, service)

		if err != nil {
			return nil, err
		}
	}

	err = l.UpdateLoadBalancer(ctx, clusterName, service, nodes)

	if err != nil {
		return nil, err
	}

	ingresses := make([]v1.LoadBalancerIngress, 0)

	for _, nic := range server.Information.NetworkInterfaces {
		for _, ip := range nic.IPAddresses {
			debugCloudAction(rtLoadBalancers, "Adding IP '%s' to load balancer ingress (name: %s)", ip.Address, loadBalancerName)

			ingresses = append(ingresses, v1.LoadBalancerIngress{
				IP: ip.Address,
			})
		}
	}

	if len(ingresses) == 0 {
		return &v1.LoadBalancerStatus{}, fmt.Errorf("No IP addresses available for load balancer (name: %s)", loadBalancerName)
	}

	return &v1.LoadBalancerStatus{Ingress: ingresses}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (l LoadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	loadBalancerName := getLoadBalancerNameByService(service)
	hostname := getLoadBalancerHostname(clusterName, loadBalancerName)

	debugCloudAction(rtLoadBalancers, "Updating load balancer (name: %s)", loadBalancerName)

	server := CloudServer{
		CloudConfiguration: l.config,
	}

	_, err := server.InitializeByHostname(hostname)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to initialize cloud server instance for load balancer (name: %s)", loadBalancerName)

		return err
	}

	if len(server.Information.NetworkInterfaces) == 0 {
		debugCloudAction(rtLoadBalancers, "Failed to find any network interfaces for load balancer (name: %s)", loadBalancerName)

		return fmt.Errorf("Cannot update load balancer due to lack of IP addresses (name: %s)", loadBalancerName)
	}

	// Retrieve the configuration values stored as annotations.
	algorithm, err := parseStringAnnotation(
		service.Annotations[annoLoadBalancerAlgorithm],
		"roundrobin",
		[]string{"leastconn", "roundrobin", "source"},
	)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to parse annotation '%s' for load balancer (name: %s)", annoLoadBalancerAlgorithm)

		return err
	}

	clientTimeout, err := parseIntAnnotation(service.Annotations[annoLoadBalancerClientTimeout], 30, 1, 86400)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to parse annotation '%s' for load balancer (name: %s)", annoLoadBalancerClientTimeout)

		return err
	}

	connectionLimit, err := parseIntAnnotation(service.Annotations[annoLoadBalancerConnectionLimit], 1000, 1, 20000)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to parse annotation '%s' for load balancer (name: %s)", annoLoadBalancerConnectionLimit)

		return err
	}

	enableProxyProtocol, _ := parseBoolAnnotation(service.Annotations[annoLoadBalancerEnableProxyProtocol], false)
	healthCheckInterval, err := parseIntAnnotation(service.Annotations[annoLoadBalancerHealthCheckInterval], 3, 3, 300)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to parse annotation '%s' for load balancer (name: %s)", annoLoadBalancerHealthCheckInterval)

		return err
	}

	healthCheckThresholdHealthy, err := parseIntAnnotation(service.Annotations[annoLoadBalancerHealthCheckThresholdHealthy], 5, 2, 10)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to parse annotation '%s' for load balancer (name: %s)", annoLoadBalancerHealthCheckThresholdHealthy)

		return err
	}

	healthCheckThresholdUnhealthy, err := parseIntAnnotation(service.Annotations[annoLoadBalancerHealthCheckThresholdUnhealthy], 3, 2, 10)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to parse annotation '%s' for load balancer (name: %s)", healthCheckThresholdUnhealthy)

		return err
	}

	healthCheckTimeout, err := parseIntAnnotation(service.Annotations[annoLoadBalancerHealthCheckTimeout], 5, 3, 300)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to parse annotation '%s' for load balancer (name: %s)", healthCheckTimeout)

		return err
	}

	serverTimeout, err := parseIntAnnotation(service.Annotations[annoLoadBalancerServerTimeout], 60, 1, 86400)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to parse annotation '%s' for load balancer (name: %s)", annoLoadBalancerServerTimeout)

		return err
	}

	// Generate a new HAProxy configuration file.
	debugCloudAction(rtLoadBalancers, "Generating new configuration file for load balancer (name: %s)", loadBalancerName)

	processorCount := getProcessorCountByConnectionLimit(connectionLimit)
	configFileContents := strings.TrimSpace(fmt.Sprintf(
		`
global
	log /dev/log local0 info alert
	log /dev/log local1 notice alert

	chroot /var/lib/haproxy

	stats socket /run/haproxy/admin.sock mode 660 level admin expose-fd listeners
	stats timeout 30s

	user haproxy
	group haproxy

	ca-base /etc/ssl/certs
	crt-base /etc/ssl/private

	ssl-default-bind-ciphers ECDH+AESGCM:DH+AESGCM:ECDH+AES256:DH+AES256:ECDH+AES128:DH+AES:RSA+AESGCM:RSA+AES:!aNULL:!MD5:!DSS
	ssl-default-bind-options no-sslv3

	nbproc %d
	nbthread 2
		`,
		processorCount,
	))

	configFileContents = configFileContents + "\n\n"

	for i := 1; i <= processorCount; i++ {
		configFileContents = configFileContents + fmt.Sprintf("\tcpu-map %d %d\n", i, i)
	}

	configFileContents = configFileContents + "\n"
	configFileContents = configFileContents + strings.TrimSpace(fmt.Sprintf(
		`
defaults
	balance %s
	log global
	maxconn %d
	mode tcp

	timeout check %ds
	timeout client %ds
	timeout connect 5s
	timeout server %ds
		`,
		algorithm,
		int(connectionLimit/processorCount),
		healthCheckTimeout,
		clientTimeout,
		serverTimeout,
	))

	configFileContents = configFileContents + "\n\n"
	serverLineFormat := "\tserver %s:%d %s:%d maxconn %d check inter %d fall %d rise %d"

	if enableProxyProtocol {
		serverLineFormat = serverLineFormat + " send-proxy"
	}

	serverLineFormat = serverLineFormat + "\n"

	for _, port := range service.Spec.Ports {
		configFileContents = configFileContents + strings.TrimSpace(fmt.Sprintf(
			`
listen %d
	bind 0.0.0.0:%d

	option tcp-check
			`,
			port.Port,
			port.Port,
		))

		configFileContents = configFileContents + "\n\n"

		for _, node := range nodes {
			for _, address := range node.Status.Addresses {
				if address.Type != "ExternalIP" {
					continue
				}

				configFileContents = configFileContents + fmt.Sprintf(
					serverLineFormat,
					address.Address,
					port.NodePort,
					address.Address,
					port.NodePort,
					int(connectionLimit/processorCount),
					healthCheckInterval,
					healthCheckThresholdUnhealthy,
					healthCheckThresholdHealthy,
				)
			}
		}
	}

	// Upload the new configuration file to the server using SFTP.
	debugCloudAction(rtLoadBalancers, "Establishing SSH connection to load balancer (name: %s)", loadBalancerName)

	sshClient, err := server.SSH()

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to establish SSH connection to load balancer (name: %s)", loadBalancerName)

		return err
	}

	defer sshClient.Close()

	debugCloudAction(rtLoadBalancers, "Creating new SFTP client for load balancer (name: %s)", loadBalancerName)

	sftp, err := sftp.NewClient(sshClient)

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to create new SFTP client for load balancer (name: %s)", loadBalancerName)

		return err
	}

	defer sftp.Close()

	debugCloudAction(rtLoadBalancers, "Uploading new configuration file to load balancer (name: %s)", loadBalancerName)

	cfgFile, err := sftp.Create("/etc/haproxy/haproxy.cfg")

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to upload new configuration file to load balancer (name: %s)", loadBalancerName)

		return err
	}

	_, err = cfgFile.Write(bytes.NewBufferString(configFileContents).Bytes())

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to upload new configuration file to load balancer (name: %s)", loadBalancerName)

		return err
	}

	// Reload the HAProxy service now that the configuration file has been updated.
	debugCloudAction(rtLoadBalancers, "Creating new SSH session for load balancer (name: %s)", loadBalancerName)

	sshSession, err := sshClient.NewSession()

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to create new SSH session for load balancer (name: %s)", loadBalancerName)

		return err
	}

	defer sshSession.Close()

	_, err = sshSession.CombinedOutput("systemctl reload haproxy")

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to load the new configuration file for load balancer (name: %s)", loadBalancerName)
	}

	return err
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it exists, returning nil if the load balancer specified either didn't exist or was successfully deleted.
// This construction is useful because many cloud providers' load balancers have multiple underlying components, meaning a Get could say that the LB doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (l LoadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	loadBalancerName := getLoadBalancerNameByService(service)
	hostname := getLoadBalancerHostname(clusterName, loadBalancerName)

	debugCloudAction(rtLoadBalancers, "Ensuring that load balancer has been deleted (name: %s)", loadBalancerName)

	server := CloudServer{
		CloudConfiguration: l.config,
	}

	notFound, err := server.InitializeByHostname(hostname)

	if err != nil {
		if notFound {
			return nil
		}

		debugCloudAction(rtLoadBalancers, "Failed to determine if load balancer exists (name: %s)", loadBalancerName)

		return err
	}

	err = server.Destroy()

	if err != nil {
		debugCloudAction(rtLoadBalancers, "Failed to destroy cloud server for load balancer (name: %s)", loadBalancerName)

		return err
	}

	return nil
}
