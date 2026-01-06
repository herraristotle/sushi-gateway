package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAggregationHandler_SingleBackend(t *testing.T) {
	// Create a mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "hello from backend"})
	}))
	defer backend.Close()

	// Create route with one backend
	route := &model.Route{
		Name: "test-route",
		Backends: []model.Backend{
			{
				Name:      "service1",
				Target:    getTarget(backend.URL),
				Path:      "/api/data",
				Protocol:  "http",
				TimeoutMs: 5000,
				Required:  true,
			},
		},
	}

	handler := NewAggregationHandler()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.HandleAggregation(w, req, route)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AggregatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 1, response.Meta.TotalBackends)
	assert.Equal(t, 1, response.Meta.SuccessfulCalls)
	assert.Equal(t, 0, response.Meta.FailedCalls)
	assert.Contains(t, response.Data, "service1")
}

func TestAggregationHandler_MultipleBackends(t *testing.T) {
	// Create mock backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"user": "john"})
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]string{"orders": {"order1", "order2"}})
	}))
	defer backend2.Close()

	backend3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"notifications": 5})
	}))
	defer backend3.Close()

	route := &model.Route{
		Name: "dashboard",
		Backends: []model.Backend{
			{Name: "user", Target: getTarget(backend1.URL), Path: "/", Protocol: "http", Required: true},
			{Name: "orders", Target: getTarget(backend2.URL), Path: "/", Protocol: "http", Required: false},
			{Name: "notifications", Target: getTarget(backend3.URL), Path: "/", Protocol: "http", Required: false},
		},
	}

	handler := NewAggregationHandler()
	req := httptest.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.HandleAggregation(w, req, route)

	assert.Equal(t, http.StatusOK, w.Code)

	var response AggregatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 3, response.Meta.TotalBackends)
	assert.Equal(t, 3, response.Meta.SuccessfulCalls)
	assert.Equal(t, 0, response.Meta.FailedCalls)
	assert.Contains(t, response.Data, "user")
	assert.Contains(t, response.Data, "orders")
	assert.Contains(t, response.Data, "notifications")
}

func TestAggregationHandler_PartialFailure(t *testing.T) {
	// Create working backend
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer backend1.Close()

	route := &model.Route{
		Name: "partial-test",
		Backends: []model.Backend{
			{Name: "working", Target: getTarget(backend1.URL), Path: "/", Protocol: "http", Required: false},
			{Name: "failing", Target: "localhost:59999", Path: "/", Protocol: "http", Required: false, TimeoutMs: 100},
		},
	}

	handler := NewAggregationHandler()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.HandleAggregation(w, req, route)

	// Should return 206 Partial Content
	assert.Equal(t, http.StatusPartialContent, w.Code)

	var response AggregatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 2, response.Meta.TotalBackends)
	assert.Equal(t, 1, response.Meta.SuccessfulCalls)
	assert.Equal(t, 1, response.Meta.FailedCalls)
	assert.Contains(t, response.Data, "working")
	assert.Len(t, response.Errors, 1)
	assert.Equal(t, "failing", response.Errors[0].Backend)
}

func TestAggregationHandler_RequiredBackendFailure(t *testing.T) {
	// Non-existent required backend
	route := &model.Route{
		Name: "required-test",
		Backends: []model.Backend{
			{Name: "critical", Target: "localhost:59999", Path: "/", Protocol: "http", Required: true, TimeoutMs: 100},
		},
	}

	handler := NewAggregationHandler()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.HandleAggregation(w, req, route)

	// Should return 502 Bad Gateway
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestAggregationHandler_Headers(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
	}))
	defer backend.Close()

	route := &model.Route{
		Name: "header-test",
		Backends: []model.Backend{
			{Name: "service", Target: getTarget(backend.URL), Path: "/", Protocol: "http"},
		},
	}

	handler := NewAggregationHandler()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.HandleAggregation(w, req, route)

	assert.Equal(t, "1", w.Header().Get("X-Aggregation-Backends"))
	assert.Equal(t, "1", w.Header().Get("X-Aggregation-Success"))
}

func TestIsAggregationRoute(t *testing.T) {
	// Route with backends
	routeWithBackends := &model.Route{
		Name: "agg-route",
		Backends: []model.Backend{
			{Name: "backend1"},
		},
	}
	assert.True(t, IsAggregationRoute(routeWithBackends))

	// Route without backends
	routeWithoutBackends := &model.Route{
		Name: "normal-route",
	}
	assert.False(t, IsAggregationRoute(routeWithoutBackends))
}

func TestAggregationHandler_BackendDurations(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer backend.Close()

	route := &model.Route{
		Name: "duration-test",
		Backends: []model.Backend{
			{Name: "slow-service", Target: getTarget(backend.URL), Path: "/", Protocol: "http"},
		},
	}

	handler := NewAggregationHandler()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	handler.HandleAggregation(w, req, route)

	var response AggregatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Check that duration was recorded
	duration, ok := response.Meta.BackendDurations["slow-service"]
	assert.True(t, ok)
	assert.GreaterOrEqual(t, duration, int64(10)) // At least 10ms
}

// Helper to get target from test server URL
func getTarget(rawURL string) string {
	target := strings.TrimPrefix(rawURL, "http://")
	return strings.TrimPrefix(target, "https://")
}
