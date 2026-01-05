package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreakerPlugin_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "missing failure_threshold",
			config:  map[string]interface{}{"timeout": float64(30)},
			wantErr: true,
		},
		{
			name:    "missing timeout",
			config:  map[string]interface{}{"failure_threshold": float64(5)},
			wantErr: true,
		},
		{
			name: "valid config",
			config: map[string]interface{}{
				"failure_threshold": float64(5),
				"timeout":           float64(30),
			},
			wantErr: false,
		},
		{
			name: "valid config with success_threshold",
			config: map[string]interface{}{
				"failure_threshold": float64(5),
				"timeout":           float64(30),
				"success_threshold": float64(2),
			},
			wantErr: false,
		},
		{
			name: "invalid success_threshold",
			config: map[string]interface{}{
				"failure_threshold": float64(5),
				"timeout":           float64(30),
				"success_threshold": float64(0),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin := CircuitBreakerPlugin{config: tt.config}
			err := plugin.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCircuitBreakerPlugin_StateTransitions(t *testing.T) {
	// Reset global state
	circuitBreakersMutex.Lock()
	circuitBreakers = make(map[string]*CircuitBreaker)
	circuitBreakersMutex.Unlock()

	config := map[string]interface{}{
		"failure_threshold": float64(3),
		"timeout":           float64(1), // 1 second for faster testing
		"success_threshold": float64(2),
	}
	plugin := CircuitBreakerPlugin{config: config}

	// Track successes and failures
	failureHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test 1: Circuit starts closed
	cb := getOrCreateCircuitBreaker("test-service")
	assert.Equal(t, CircuitClosed, cb.state)

	// Test 2: Record failures until circuit opens
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test-service/api", nil)
		rr := httptest.NewRecorder()
		plugin.Execute(failureHandler).ServeHTTP(rr, req)
	}
	assert.Equal(t, CircuitOpen, cb.state)

	// Test 3: Requests are rejected when circuit is open
	req := httptest.NewRequest("GET", "/test-service/api", nil)
	rr := httptest.NewRecorder()
	plugin.Execute(successHandler).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

	// Test 4: Wait for timeout, circuit should transition to half-open
	time.Sleep(1100 * time.Millisecond)
	req = httptest.NewRequest("GET", "/test-service/api", nil)
	rr = httptest.NewRecorder()
	plugin.Execute(successHandler).ServeHTTP(rr, req)
	assert.Equal(t, CircuitHalfOpen, cb.state)

	// Test 5: Success in half-open increases success count
	req = httptest.NewRequest("GET", "/test-service/api", nil)
	rr = httptest.NewRecorder()
	plugin.Execute(successHandler).ServeHTTP(rr, req)
	// After 2 successes (threshold), circuit should close
	assert.Equal(t, CircuitClosed, cb.state)
}

func TestIsFailureStatus(t *testing.T) {
	assert.True(t, isFailureStatus(500))
	assert.True(t, isFailureStatus(502))
	assert.True(t, isFailureStatus(503))
	assert.True(t, isFailureStatus(504))
	assert.False(t, isFailureStatus(200))
	assert.False(t, isFailureStatus(404))
	assert.False(t, isFailureStatus(400))
}
