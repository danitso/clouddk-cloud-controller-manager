package clouddkcp

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	// annoLoadBalancerAlgorithm is the annotation specifying which load balancing algorithm to use.
	// Options are leastconn, roundrobin and source. Defaults to roundrobin.
	annoLoadBalancerAlgorithm = "kubernetes.cloud.dk/load-balancer-algorithm"

	// annoLoadBalancerAlgorithm is the annotation specifying the connection limit.
	annoLoadBalancerConnectionLimit = "kubernetes.cloud.dk/load-balancer-connection-limit"

	// annoLoadBalancerEnableProxyProtocol is the annotation specifying whether the PROXY protocol should be enabled.
	// Defaults to false.
	annoLoadBalancerEnableProxyProtocol = "kubernetes.cloud.dk/load-balancer-enable-proxy-protocol"

	// annoLoadBalancerEnableHTTPSRedirection is the annotation specifying whether or not HTTP traffic should be redirected to HTTPS.
	// Defaults to false
	annoLoadBalancerEnableHTTPSRedirection = "kubernetes.cloud.dk/load-balancer-enable-https-redirection"

	// annoLoadBalancerHealthCheckInternal is the annotation used to specify the number of seconds between between two consecutive health checks.
	// The value must be between 3 and 300. Defaults to 3.
	annoLoadBalancerHealthCheckInterval = "kubernetes.cloud.dk/load-balancer-health-check-interval"

	// annoLoadBalancerHealthCheckPath is the annotation used to specify the health check path.
	// Defaults to '/'.
	annoLoadBalancerHealthCheckPath = "kubernetes.cloud.dk/load-balancer-health-check-path"

	// annoLoadBalancerHealthCheckProtocol is the annotation used to specify the health check protocol.
	// Defaults to the protocol used in 'kubernetes.cloud.dk/load-balancer-protocol'.
	annoLoadBalancerHealthCheckProtocol = "kubernetes.cloud.dk/load-balancer-health-check-protocol"

	// annoLoadBalancerHealthCheckThresholdHealthy is the annotation used to specify the number of times a health check must pass for a backend to be marked "healthy" for the given service and be re-added to the pool.
	// The value must be between 2 and 10. Defaults to 5.
	annoLoadBalancerHealthCheckThresholdHealthy = "kubernetes.cloud.dk/load-balancer-health-check-threshold-healthy"

	// annoLoadBalancerHealthCheckThresholdUnhealthy is the annotation used to specify the number of times a health check must fail for a backend to be marked "unhealthy" and be removed from the pool for the given service.
	// The value must be between 2 and 10. Defaults to 3.
	annoLoadBalancerHealthCheckThresholdUnhealthy = "kubernetes.cloud.dk/load-balancer-health-check-threshold-unhealthy"

	// annoLoadBalancerHealthCheckTimeout is the annotation used to specify the number of seconds the Load Balancer will wait for a response until marking a health check as failed.
	// The value must be between 3 and 300. Defaults to 5.
	annoLoadBalancerHealthCheckTimeout = "kubernetes.cloud.dk/load-balancer-health-check-timeout"

	// annoLoadBalancerID is the annotation specifying the load balancer ID used to enable fast retrievals of load balancers from the API.
	annoLoadBalancerID = "kubernetes.cloud.dk/load-balancer-id"

	// annoLoadBalancerProtocol is the annotation used to specify the default protocol for load balancers.
	// For ports specified in annoLoadBalancerTLSPorts, this protocol is overwritten to https.
	// Options are tcp, http and https. Defaults to tcp.
	annoLoadBalancerProtocol = "kubernetes.cloud.dk/load-balancer-protocol"

	// annoLoadBalancerTLSPassthrough is the annotation used to specify whether the load balancer should pass encrypted data to backend servers.
	// This is optional and defaults to false.
	annoLoadBalancerTLSPassthrough = "kubernetes.cloud.dk/load-balancer-tls-passthrough"

	// annoLoadBalancerTLSPorts is the annotation used to specify which ports of the load balancer should use the HTTPS protocol.
	// This is a comma separated list of ports (e.g., 443,6443,7443).
	annoLoadBalancerTLSPorts = "kubernetes.cloud.dk/load-balancer-tls-ports"

	// annoLoadBalancerTLSSecret is the annotation used to specify the name of the secret which contains the SSL certificate and key.
	annoLoadBalancerTLSSecret = "kubernetes.cloud.dk/load-balancer-tls-secret"

	// annoLoadBalancerStickySessionsCookieName is the annotation specifying what cookie name to use for sticky sessions.
	// This annotation is required, if annoLoadBalancerStickySessionsType is set to cookies.
	annoLoadBalancerStickySessionsCookieName = "kubernetes.cloud.dk/load-balancer-sticky-sessions-cookie-name"

	// annoLoadBalancerStickySessionsTTL is the annotation specifying the TTL of the cookie used for sticky sessions.
	// This annotation is required, if annoLoadBalancerStickySessionsType is set to cookies.
	annoLoadBalancerStickySessionsTTL = "kubernetes.cloud.dk/load-balancer-sticky-sessions-ttl"

	// annoLoadBalancerStickySessionsType is the annotation specifying which sticky session type to use.
	// Options are none and cookies. Defaults to none.
	annoLoadBalancerStickySessionsType = "kubernetes.cloud.dk/load-balancer-sticky-sessions-type"

	// hostnameFormat specifies the format for load balancer hostnames.
	hostnameFormat = "k8s-load-balancer-%s-%s"
)

// LoadBalancers implements the interface cloudprovider.LoadBalancer
type LoadBalancers struct {
	config *CloudConfiguration
}

// createLoadBalancer creates a new load balancer
func createLoadBalancer(c *CloudConfiguration, hostname string, service *v1.Service) (CloudServer, error) {
	server := CloudServer{
		CloudConfiguration: c,
	}

	connectionLimit := service.Annotations[annoLoadBalancerConnectionLimit]
	packageID := ""

	if connectionLimit != "" {
		limit, err := strconv.Atoi(connectionLimit)

		if err != nil {
			return server, fmt.Errorf("Invalid connection limit '%s' (%s)", connectionLimit, err.Error())
		}

		packageID = getPackageIDByConnectionLimit(limit)
	} else {
		packageID = getPackageIDByConnectionLimit(1000)
	}

	serverCreateErr := server.Create("dk1", packageID, hostname)

	if serverCreateErr != nil {
		return server, serverCreateErr
	}

	// Install an LTS version of HAProxy on the server.
	sshClient, sshClientErr := server.SSH()

	if sshClientErr != nil {
		return server, sshClientErr
	}

	sshSession, sshSessionErr := sshClient.NewSession()

	if sshSessionErr != nil {
		sshClient.Close()
		server.Destroy()

		return server, sshSessionErr
	}

	_, sshOuputErr := sshSession.CombinedOutput(
		"export DEBIAN_FRONTEND=noninteractive && " +
			"apt-get -qq update && " +
			"apt-get -qq install -y software-properties-common && " +
			"add-apt-repository -y ppa:vbernat/haproxy-2.0 && " +
			"apt-get -qq install -y haproxy=2.0.\\*",
	)

	if sshOuputErr != nil {
		sshClient.Close()
		server.Destroy()

		return server, sshSessionErr
	}

	sshClient.Close()

	return server, nil
}

// getLoadBalancerNameByService retrieves the default load balancer name for a service
func getLoadBalancerNameByService(service *v1.Service) string {
	name := strings.Replace(string(service.UID), "-", "", -1)

	if len(name) > 32 {
		name = name[:32]
	}

	return name
}

// getPackageIDByConnectionLimit retrieves the package id based on a connection limit
func getPackageIDByConnectionLimit(limit int) string {
	if limit <= 1000 {
		return "89833c1dfa7010"
	} else if limit <= 10000 {
		return "e991abd8ef15c7"
	} else {
		return "9559dbb4b71c45"
	}
}

// getProcessorCountByConnectionLimit retrieves the package id based on a connection limit
func getProcessorCountByConnectionLimit(limit int) int {
	if limit <= 1000 {
		return 1
	} else if limit <= 10000 {
		return 2
	} else {
		return 4
	}
}

// newLoadBalancers initializes a new LoadBalancers object
func newLoadBalancers(c *CloudConfiguration) cloudprovider.LoadBalancer {
	return LoadBalancers{
		config: c,
	}
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
	return status, false, nil
}

// GetLoadBalancerName returns the name of the load balancer.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
func (l LoadBalancers) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	return getLoadBalancerNameByService(service)
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l LoadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	hostname := fmt.Sprintf(hostnameFormat, sanitizeClusterName(clusterName), getLoadBalancerNameByService(service))

	server := CloudServer{
		CloudConfiguration: l.config,
	}

	serverErr := server.GetByHostname(hostname)

	if serverErr != nil {
		server, serverErr = createLoadBalancer(l.config, hostname, service)

		if serverErr != nil {
			return nil, serverErr
		}
	}

	updateErr := l.UpdateLoadBalancer(ctx, clusterName, service, nodes)

	if updateErr != nil {
		return nil, updateErr
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: server.Information.NetworkInterfaces[0].IPAddresses[0].Address,
			},
		},
	}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l LoadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	hostname := fmt.Sprintf(hostnameFormat, sanitizeClusterName(clusterName), getLoadBalancerNameByService(service))

	server := CloudServer{
		CloudConfiguration: l.config,
	}

	serverErr := server.GetByHostname(hostname)

	if serverErr != nil {
		return serverErr
	}

	// Retrieve the configuration values stored as annotations.
	algorithm := service.Annotations[annoLoadBalancerAlgorithm]

	if algorithm == "" {
		algorithm = "roundrobin"
	}

	switch algorithm {
	case "leastconn":
	case "roundrobin":
	case "source":
		break
	default:
		return fmt.Errorf("Invalid algorithm '%s'", algorithm)
	}

	var connectionLimitErr error

	connectionLimit := 1000
	connectionLimitStr := service.Annotations[annoLoadBalancerConnectionLimit]

	if connectionLimitStr != "" {
		connectionLimit, connectionLimitErr = strconv.Atoi(connectionLimitStr)

		if connectionLimitErr != nil {
			return fmt.Errorf("Invalid connection limit '%s'", connectionLimitStr)
		}
	}

	if connectionLimit < 1 {
		return fmt.Errorf("Invalid connection limit '%s'", connectionLimitStr)
	}

	healthCheckInterval := service.Annotations[annoLoadBalancerHealthCheckInterval]

	if healthCheckInterval == "" {
		healthCheckInterval = "3"
	}

	healthCheckPath := service.Annotations[annoLoadBalancerHealthCheckPath]

	if healthCheckPath == "" {
		healthCheckPath = "/"
	}

	healthCheckThresholdHealthy := service.Annotations[annoLoadBalancerHealthCheckThresholdHealthy]

	if healthCheckThresholdHealthy == "" {
		healthCheckThresholdHealthy = "5"
	}

	healthCheckThresholdUnhealthy := service.Annotations[annoLoadBalancerHealthCheckThresholdUnhealthy]

	if healthCheckThresholdUnhealthy == "" {
		healthCheckThresholdUnhealthy = "3"
	}

	healthCheckTimeout := service.Annotations[annoLoadBalancerHealthCheckTimeout]

	if healthCheckTimeout == "" {
		healthCheckTimeout = "5"
	}

	protocol := service.Annotations[annoLoadBalancerProtocol]

	if protocol == "" {
		protocol = "tcp"
	}

	healthCheckProtocol := service.Annotations[annoLoadBalancerHealthCheckProtocol]

	if healthCheckProtocol == "" {
		healthCheckProtocol = protocol
	}

	// Generate a new HAProxy configuration file.
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
	mode %s
		`,
		algorithm,
		int(connectionLimit/processorCount),
		protocol,
	))

	configFileContents = configFileContents + "\n\n"

	for _, port := range service.Spec.Ports {
		configFileContents = configFileContents + strings.TrimSpace(fmt.Sprintf(
			`
listen p%d
	bind 0.0.0.0:%d
	timeout check %s
			`,
			port.Port,
			port.Port,
			healthCheckTimeout,
		))

		if healthCheckProtocol == "http" {
			configFileContents = configFileContents + fmt.Sprintf("\toption httpchk GET %s HTTP/1.0\n", healthCheckPath)
		} else if healthCheckProtocol == "https" {
			configFileContents = configFileContents + "\toption ssl-hello-chk\n"
		} else {
			configFileContents = configFileContents + "\toption tcp-check\n"
		}

		configFileContents = configFileContents + "\n\n"

		for _, node := range nodes {
			for _, address := range node.Status.Addresses {
				if address.Type != "ExternalIP" {
					continue
				}

				configFileContents = configFileContents + fmt.Sprintf(
					"\tserver %s:%d %s:%d maxconn %d check inter %s fall %s rise %s\n",
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

	// Upload the new configuration file to the server and reload the HAProxy service.
	sshClient, sshClientErr := server.SSH()

	if sshClientErr != nil {
		return sshClientErr
	}

	_, sshSessionErr := sshClient.NewSession()

	if sshSessionErr != nil {
		sshClient.Close()

		return sshSessionErr
	}

	sshClient.Close()

	// ... Work in progress ...

	return nil
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it exists, returning nil if the load balancer specified either didn't exist or was successfully deleted.
// This construction is useful because many cloud providers' load balancers have multiple underlying components, meaning a Get could say that the LB doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l LoadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	hostname := fmt.Sprintf(hostnameFormat, sanitizeClusterName(clusterName), getLoadBalancerNameByService(service))

	server := CloudServer{
		CloudConfiguration: l.config,
	}

	serverErr := server.GetByHostname(hostname)

	if serverErr != nil {
		return nil
	}

	serverErr = server.Destroy()

	if serverErr != nil {
		return serverErr
	}

	return nil
}
