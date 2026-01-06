package gateway

import (
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/container"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// MockRegistry implements discovery.Registry for testing
type MockRegistry struct {
	services map[string]string
}

func (m *MockRegistry) GetServiceURL(serviceName string) (string, error) {
	if url, ok := m.services[serviceName]; ok {
		return url, nil
	}
	return "", errors.New("service not found")
}

func (m *MockRegistry) Name() string {
	return "mock"
}

func TestSelectUpstream_ServiceDiscovery(t *testing.T) {
	// Setup Container with Mock Registry
	mockRegistry := &MockRegistry{
		services: map[string]string{
			"user-service": "http://10.0.0.1:8080",
			"db-service":   "postgres://db-host:5432",
		},
	}

	container.Global = &container.Container{
		Registry: mockRegistry,
		// Needs HealthChecker for LoadBalancer init inside selectUpstream
		// We can mock it or provide nil if LoadBalancer handles nil (it generally doesn't)
		// Actually NewLoadBalancer takes GlobalHealthChecker which is a global variable in gateway package
	}

	// Create Proxy Instance
	proxy := &SushiProxy{}

	// Test Case 1: Successful Discovery
	service := model.Service{
		Name:        "user-service-proxy",
		ServiceName: "user-service",
	}
	req := httptest.NewRequest("GET", "/api/users", nil)

	_, url, _, err := proxy.selectUpstream(&service, req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if url != "http://10.0.0.1:8080" {
		t.Errorf("Expected http://10.0.0.1:8080, got %s", url)
	}

	// Test Case 2: Service Not Found
	serviceUnknown := model.Service{
		Name:        "unknown-service",
		ServiceName: "unknown",
	}

	_, _, _, errUnknown := proxy.selectUpstream(&serviceUnknown, req)
	if errUnknown == nil {
		t.Error("Expected error for unknown service")
	}

	// Test Case 3: Fallback to URL (Kong style) if ServiceName not set
	serviceStatic := model.Service{
		Name: "static-service",
		URL:  "http://static:9090",
	}

	_, urlStatic, _, errStatic := proxy.selectUpstream(&serviceStatic, req)
	if errStatic != nil {
		t.Errorf("Unexpected error: %v", errStatic)
	}
	if urlStatic != "http://static:9090" {
		t.Errorf("Expected http://static:9090, got %s", urlStatic)
	}
}
