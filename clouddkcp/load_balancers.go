package clouddkcp

import (
	"context"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	// annoLoadBalancerAlgorithm is the annotation specifying which load balancing algorithm to use.
	// Options are round_robin and least_connections. Defaults to round_robin.
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
	annoLoadBalancerHealthCheckInternal = "kubernetes.cloud.dk/load-balancer-health-check-interval"

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
)

// LoadBalancers implements the interface cloudprovider.LoadBalancer
type LoadBalancers struct {
	config *CloudConfiguration
}

// newLoadBalancers initializes a new LoadBalancers object
func newLoadBalancers(c *CloudConfiguration) cloudprovider.LoadBalancer {
	return LoadBalancers{
		config: c,
	}
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
	return ""
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l LoadBalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	return nil, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l LoadBalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	return nil
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it exists, returning nil if the load balancer specified either didn't exist or was successfully deleted.
// This construction is useful because many cloud providers' load balancers have multiple underlying components, meaning a Get could say that the LB doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l LoadBalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	return nil
}
