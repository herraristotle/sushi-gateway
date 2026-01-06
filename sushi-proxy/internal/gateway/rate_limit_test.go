package gateway

import (
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

func TestRateLimitValidation(t *testing.T) {
	mockProxyConfig := createMockProxyConfig(t)

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"second": float64(10),
				"minute": float64(10),
				"hour":   float64(10),
			},
			expectError: false,
		},
		{
			name: "missing limit_second",
			config: map[string]interface{}{
				"minute": float64(10),
				"hour":   float64(10),
			},
			expectError: false,
		},
		{
			name: "missing limit_min",
			config: map[string]interface{}{
				"second": float64(10),
				"hour":   float64(10),
			},
			expectError: false,
		},
		{
			name: "missing limit_hour",
			config: map[string]interface{}{
				"second": float64(10),
				"minute": float64(10),
			},
			expectError: false,
		},
		{
			name: "invalid limit_second type",
			config: map[string]interface{}{
				"second": "10",
				"minute": float64(10),
				"hour":   float64(10),
			},
			expectError: true,
		},
		{
			name: "invalid limit_min type",
			config: map[string]interface{}{
				"second": float64(10),
				"minute": "10",
				"hour":   float64(10),
			},
			expectError: true,
		},
		{
			name: "invalid limit_hour type",
			config: map[string]interface{}{
				"second": float64(10),
				"minute": float64(10),
				"hour":   "10",
			},
			expectError: true,
		},
		{
			name: "zero limit_second",
			config: map[string]interface{}{
				"second": float64(0),
				"minute": float64(10),
				"hour":   float64(10),
			},
			expectError: true,
		},
		{
			name: "zero limit_min",
			config: map[string]interface{}{
				"second": float64(10),
				"minute": float64(0),
				"hour":   float64(10),
			},
			expectError: true,
		},
		{
			name: "zero limit_hour",
			config: map[string]interface{}{
				"second": float64(10),
				"minute": float64(10),
				"hour":   float64(0),
			},
			expectError: true,
		},
		{
			name: "negative limit_second",
			config: map[string]interface{}{
				"second": float64(-10),
				"minute": float64(10),
				"hour":   float64(10),
			},
			expectError: true,
		},
		{
			name: "negative limit_min",
			config: map[string]interface{}{
				"second": float64(10),
				"minute": float64(-10),
				"hour":   float64(10),
			},
			expectError: true,
		},
		{
			name: "negative limit_hour",
			config: map[string]interface{}{
				"second": float64(10),
				"minute": float64(10),
				"hour":   float64(-10),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pluginConfig, err := CreatePluginConfigInput(tt.config)
			if err != nil {
				t.Fatal(err)
			}

			plugin := NewRateLimitPlugin(pluginConfig, mockProxyConfig)
			err = plugin.Validator.Validate()

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func createMockProxyConfig(t *testing.T) *model.ProxyConfig {
	rateLimitPlugin := model.PluginConfig{
		Id:   "someId",
		Name: "rate-limiting",
		Config: map[string]interface{}{
			"second": 10,
			"minute": 10,
			"hour":   10,
		},
	}

	service := model.Service{
		Name:                  "mockService",
		BasePath:              "/mockService",
		Protocol:              "http",
		LoadBalancingStrategy: model.RoundRobin,
		Upstreams:             make([]model.UpstreamTarget, 0),
		Plugins: []model.PluginConfig{
			rateLimitPlugin,
		},
		Routes: []model.Route{
			{
				Name:    "mockRoute",
				Path:    "/mockRoute",
				Methods: []string{"GET"},
				Plugins: make([]model.PluginConfig, 0),
			},
		},
	}

	proxyConfig := &model.ProxyConfig{}
	proxyConfig.Name = "mockProxy"
	proxyConfig.Services = []model.Service{service}

	return proxyConfig
}
