package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

func TestDynamicRouterUpdates(t *testing.T) {
	// 1. Init Proxy
	proxy := NewSushiProxy()
	proxy.UpdateRouter(&model.ProxyConfig{}) // Empty

	// 2. Verify 404
	req := httptest.NewRequest("GET", "/api/v1/foo", nil)
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}

	// 3. Update with Route
	config := &model.ProxyConfig{
		Services: []model.Service{
			{
				Name: "test-service",
				Routes: []model.Route{
					{
						Name:  "test-route",
						Paths: []string{"/api/v1/foo"},
					},
				},
			},
		},
	}
	proxy.UpdateRouter(config)

	// 4. Verify Match
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	// If it routed, it should be 502 (Bad Gateway) because no upstream is defined/healthy.
	// IT SHOULD NOT BE 404.
	if w.Code == http.StatusNotFound {
		t.Errorf("Expected route match (non-404), got 404")
	}

	// Optional: verify that it is NOT panic.
}
