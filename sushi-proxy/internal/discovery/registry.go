package discovery

// Registry interface abstracts the service discovery mechanism
type Registry interface {
	// GetServiceURL resolves a service name to a URL (e.g., "http://10.0.0.1:8080")
	GetServiceURL(serviceName string) (string, error)

	// Name returns the name of the registry (e.g., "consul", "etcd")
	Name() string
}
