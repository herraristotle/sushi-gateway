package consul

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/hashicorp/consul/api"
)

type ConsulRegistry struct {
	client *api.Client
}

func NewConsulRegistry(address string) (*ConsulRegistry, error) {
	config := api.DefaultConfig()
	if address != "" {
		config.Address = address
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consul client: %w", err)
	}

	return &ConsulRegistry{
		client: client,
	}, nil
}

func (r *ConsulRegistry) Name() string {
	return "consul"
}

// GetServiceURL resolves a service name to a URL
// It currently uses simple random load balancing client-side
func (r *ConsulRegistry) GetServiceURL(serviceName string) (string, error) {
	// Query passing services only
	entries, _, err := r.client.Health().Service(serviceName, "", true, nil)
	if err != nil {
		return "", fmt.Errorf("consul lookup failed: %w", err)
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("service not found: %s", serviceName)
	}

	// Simple Load Balancing: Random Selection
	// In production, you might want Round-Robin or Least-Connection
	// But usually, the Registry just returns A list, and the LB handles selection.
	// For this Phase 1 integration, we select one valid instance.
	selected := entries[rand.Intn(len(entries))]

	host := selected.Service.Address
	// If address is empty, fallback to node address
	if host == "" {
		host = selected.Node.Address
	}

	port := selected.Service.Port

	// Determine protocol (metadata or default to http)
	protocol := "http"
	if p, ok := selected.Service.Meta["protocol"]; ok {
		protocol = p
	}

	url := fmt.Sprintf("%s://%s:%d", protocol, host, port)

	slog.Debug("Consul resolved service",
		"service", serviceName,
		"resolved_url", url,
		"instances_found", len(entries))

	return url, nil
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
