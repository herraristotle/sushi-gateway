package gateway

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestHealthChecker_CheckHealthForAllServices(t *testing.T) {
	// Create test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()
	server1Host, server1Port, err := util.GetHostAndPortFromURL(server1.URL)
	if err != nil {
		t.Fatalf("Failed to get host and port from server1 URL: %v", err)
	}

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server2.Close()
	server2Host, server2Port, err := util.GetHostAndPortFromURL(server2.URL)
	if err != nil {
		t.Fatalf("Failed to get host and port from server2 URL: %v", err)
	}

	// Setup test services
	config := model.ProxyConfig{
		Services: []model.Service{
			{
				Name:     "test-service-1",
				Protocol: "http",
				Upstreams: []model.UpstreamTarget{
					{Id: "upstream1", Target: server1Host + ":" + strconv.Itoa(server1Port)},
					{Id: "upstream2", Target: server2Host + ":" + strconv.Itoa(server2Port)},
				},
				Health: model.Health{
					Enabled: true,
					Path:    "/health",
				},
			},
		},
	}
	globalProxyConfig.Store(&config)

	hc := NewHealthChecker()
	hc.Initialize()
	hc.CheckHealthForAllServices()

	// Verify results
	assert.Equal(t, Healthy, hc.serviceHealthMap["test-service-1"]["upstream1"].Status)
	assert.Equal(t, Unhealthy, hc.serviceHealthMap["test-service-1"]["upstream2"].Status)
}

func TestHealthChecker_GetHealthyUpstreams(t *testing.T) {
	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "127.0.0.1:8081"},
			{Id: "upstream2", Target: "127.0.0.1:8082"},
			{Id: "upstream3", Target: "localhost:8083"},
		},
		Health: model.Health{
			Enabled: true,
			Path:    "/health",
		},
	}

	tests := []struct {
		name           string
		healthStatuses map[string]map[string]*UpstreamHealthState
		expectedCount  int
	}{
		{
			name: "All upstreams healthy",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Healthy},
					"upstream2": {Status: Healthy},
					"upstream3": {Status: Healthy},
				},
			},
			expectedCount: 3,
		},
		{
			name: "Mixed health status",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Healthy},
					"upstream2": {Status: Unhealthy},
					"upstream3": {Status: Healthy},
				},
			},
			expectedCount: 2,
		},
		{
			name: "All upstreams unhealthy",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Unhealthy},
					"upstream2": {Status: Unhealthy},
					"upstream3": {Status: Unhealthy},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Some upstreams not available",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Healthy},
					"upstream2": {Status: NotAvailable},
					"upstream3": {Status: Healthy},
				},
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := &HealthChecker{
				serviceHealthMap: tt.healthStatuses,
			}

			healthyUpstreams := hc.GetHealthyUpstreams(service)
			assert.Equal(t, tt.expectedCount, len(healthyUpstreams))
		})
	}
}

func TestHealthChecker_Initialize(t *testing.T) {
	// Setup test configuration
	config := model.ProxyConfig{
		Services: []model.Service{
			{
				Name: "test-service-1",
				Upstreams: []model.UpstreamTarget{
					{Id: "upstream1", Target: "localhost:8081"},
					{Id: "upstream2", Target: "localhost:8082"},
				},
			},
			{
				Name: "test-service-2",
				Upstreams: []model.UpstreamTarget{
					{Id: "upstream3", Target: "localhost:8083"},
				},
			},
		},
	}
	globalProxyConfig.Store(&config)

	hc := NewHealthChecker()
	hc.Initialize()

	// Verify initialization
	assert.Contains(t, hc.serviceHealthMap, "test-service-1")
	assert.Contains(t, hc.serviceHealthMap, "test-service-2")
	assert.Equal(t, NotAvailable, hc.serviceHealthMap["test-service-1"]["upstream1"].Status)
	assert.Equal(t, NotAvailable, hc.serviceHealthMap["test-service-1"]["upstream2"].Status)
	assert.Equal(t, NotAvailable, hc.serviceHealthMap["test-service-2"]["upstream3"].Status)
}

func TestHealthChecker_UpdateHealthState(t *testing.T) {
	hc := NewHealthChecker()
	// Setup test configuration
	config := model.ProxyConfig{
		Services: []model.Service{
			{
				Name: "test-service",
				Upstreams: []model.UpstreamTarget{
					{Id: "upstream1", Target: "localhost:8081"},
					{Id: "upstream2", Target: "localhost:8082"},
				},
			},
		},
	}
	globalProxyConfig.Store(&config)

	hc.Initialize()

	// Test updating health status
	hc.UpdateHealthState("test-service", "upstream1", &UpstreamHealthState{Status: Healthy})
	assert.Equal(t, Healthy, hc.serviceHealthMap["test-service"]["upstream1"].Status)

	// Test updating to unhealthy
	hc.UpdateHealthState("test-service", "upstream1", &UpstreamHealthState{Status: Unhealthy})
	assert.Equal(t, Unhealthy, hc.serviceHealthMap["test-service"]["upstream1"].Status)
}

func TestHealthChecker_DisabledHealthCheck(t *testing.T) {
	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "127.0.0.1:8081"},
		},
		Health: model.Health{
			Enabled: false,
			Path:    "/health",
		},
	}

	hc := NewHealthChecker()
	hc.Initialize()

	// GetHealthyUpstreams should return all upstreams when health check is disabled
	healthyUpstreams := hc.GetHealthyUpstreams(service)
	assert.Equal(t, len(service.Upstreams), len(healthyUpstreams))
}
