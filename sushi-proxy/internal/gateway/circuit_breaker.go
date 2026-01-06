package gateway

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/sony/gobreaker"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// CircuitBreaker wrapper around gobreaker
type CircuitBreaker struct {
	cb *gobreaker.CircuitBreaker
}

// Global circuit breakers map - keyed by service name
var circuitBreakers = make(map[string]*CircuitBreaker)
var circuitBreakersMutex = sync.RWMutex{}

// getOrCreateCircuitBreaker gets or creates a circuit breaker for a service
// We need the config here to initialize it properly if it doesn't exist.
// Since we might not have config when just "getting", this design assumes
// initialization happens via middleware or we use defaults.
// For the plugin architecture, we'll initialize with defaults if missing,
// but the Execute method will have access to the config to update/re-create if needed
// (though re-creating is expensive/tricky for state).
// A better approach for this plugin: we use a single config per service.
func getOrCreateCircuitBreaker(serviceName string, failureThreshold int, successThreshold int, timeout time.Duration) *CircuitBreaker {
	circuitBreakersMutex.Lock()
	defer circuitBreakersMutex.Unlock()

	if cb, exists := circuitBreakers[serviceName]; exists {
		return cb
	}

	st := gobreaker.Settings{
		Name:        serviceName,
		MaxRequests: uint32(successThreshold), // Half-open success threshold
		Interval:    0,                        // Clear counts only on success/failure, not time
		Timeout:     timeout,                  // Open -> Half-Open timeout
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip when failures >= threshold
			shouldTrip := counts.ConsecutiveFailures >= uint32(failureThreshold)
			if shouldTrip {
				slog.Warn("Circuit breaker tripping", "service", serviceName, "failures", counts.ConsecutiveFailures)
			}
			return shouldTrip
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			slog.Info("Circuit breaker state change", "service", name, "from", from, "to", to)
		},
	}

	cb := &CircuitBreaker{
		cb: gobreaker.NewCircuitBreaker(st),
	}
	circuitBreakers[serviceName] = cb
	return cb
}

type CircuitBreakerPlugin struct {
	config map[string]interface{}
}

func NewCircuitBreakerPlugin(config map[string]interface{}) *Plugin {
	return &Plugin{
		Name:     constant.PLUGIN_CIRCUIT_BREAKER,
		Priority: 100, // Execute early to fail fast
		Handler: CircuitBreakerPlugin{
			config: config,
		},
		Validator: CircuitBreakerPlugin{
			config: config,
		},
	}
}

func (plugin CircuitBreakerPlugin) Validate() error {
	// failure_threshold is required
	failureThreshold, ok := plugin.config["failure_threshold"].(float64)
	if !ok || failureThreshold < 1 {
		return fmt.Errorf("failure_threshold must be a positive integer")
	}

	// timeout is required (seconds before trying half-open)
	timeout, ok := plugin.config["timeout"].(float64)
	if !ok || timeout < 1 {
		return fmt.Errorf("timeout must be a positive integer (seconds)")
	}

	// success_threshold is optional, defaults to 1
	if successThreshold, ok := plugin.config["success_threshold"].(float64); ok {
		if successThreshold < 1 {
			return fmt.Errorf("success_threshold must be a positive integer")
		}
	}

	return nil
}

func (plugin CircuitBreakerPlugin) getConfig() (failureThreshold int, successThreshold int, timeout time.Duration) {
	failureThreshold = int(plugin.config["failure_threshold"].(float64))
	timeout = time.Duration(int(plugin.config["timeout"].(float64))) * time.Second

	// Default success threshold to 1
	successThreshold = 1
	if st, ok := plugin.config["success_threshold"].(float64); ok {
		successThreshold = int(st)
	}

	return
}

func (plugin CircuitBreakerPlugin) Execute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get service name from request context or path
		serviceName := getServiceNameFromRequest(r)

		failureThreshold, successThreshold, timeout := plugin.getConfig()

		// Get or create the CP with the current config
		cb := getOrCreateCircuitBreaker(serviceName, failureThreshold, successThreshold, timeout)

		var recorder *responseRecorder

		// Execute the request via the circuit breaker
		_, err := cb.cb.Execute(func() (interface{}, error) {
			// Create a response recorder to capture the response status
			recorder = &responseRecorder{
				headers:    make(http.Header),
				body:       new(bytes.Buffer),
				statusCode: http.StatusOK,
			}

			// Execute the next handler
			next.ServeHTTP(recorder, r)

			// Check if the response was a failure
			if isFailureStatus(recorder.statusCode) {
				return nil, fmt.Errorf("upstream failure: %d", recorder.statusCode)
			}

			return nil, nil
		})

		// Handle circuit breaker errors (Open state or Too Many Requests)
		if err == gobreaker.ErrOpenState || err == gobreaker.ErrTooManyRequests {
			slog.Warn("Circuit breaker rejected request", "service", serviceName, "error", err)
			model.NewHttpError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE",
				"Service temporarily unavailable (circuit breaker open)").WriteJSONResponse(w)
			return
		}

		// For success or upstream failure (which we trapped to update CB stats),
		// we write the buffered response to the client
		if recorder != nil {
			// Copy headers
			for k, v := range recorder.headers {
				for _, val := range v {
					w.Header().Add(k, val)
				}
			}
			// Write status code
			w.WriteHeader(recorder.statusCode)
			// Write body
			if recorder.body != nil {
				w.Write(recorder.body.Bytes())
			}
		}
	})
}

// responseRecorder buffers the response to avoid writing until CB logic is done
type responseRecorder struct {
	headers    http.Header
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Header() http.Header {
	return r.headers
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
}

// isFailureStatus checks if the HTTP status code indicates a failure
func isFailureStatus(code int) bool {
	return code >= 500 && code <= 599
}

// getServiceNameFromRequest extracts service name from request
func getServiceNameFromRequest(r *http.Request) string {
	// Try to get from context first (set by router)
	if serviceName, ok := r.Context().Value("service_name").(string); ok {
		return serviceName
	}
	// Fallback to first path segment
	path := r.URL.Path
	if len(path) > 1 {
		// Remove leading slash and get first segment
		parts := splitPath(path)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return "default"
}

func splitPath(path string) []string {
	var parts []string
	start := 0
	if len(path) > 0 && path[0] == '/' {
		start = 1
	}
	for i := start; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				parts = append(parts, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		parts = append(parts, path[start:])
	}
	return parts
}
