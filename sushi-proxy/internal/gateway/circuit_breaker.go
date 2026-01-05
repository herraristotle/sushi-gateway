package gateway

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/constant"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// CircuitState represents the state of the circuit breaker
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal operation, requests flow through
	CircuitOpen                         // Failing, reject requests immediately
	CircuitHalfOpen                     // Testing recovery, allow limited requests
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker tracks the state of each service's circuit
type CircuitBreaker struct {
	state           CircuitState
	failures        int
	successes       int
	lastFailureTime time.Time
	mutex           sync.RWMutex
}

// Global circuit breakers map - keyed by service name
var circuitBreakers = make(map[string]*CircuitBreaker)
var circuitBreakersMutex = sync.RWMutex{}

// getOrCreateCircuitBreaker gets or creates a circuit breaker for a service
func getOrCreateCircuitBreaker(serviceName string) *CircuitBreaker {
	circuitBreakersMutex.Lock()
	defer circuitBreakersMutex.Unlock()

	if cb, exists := circuitBreakers[serviceName]; exists {
		return cb
	}

	cb := &CircuitBreaker{
		state:    CircuitClosed,
		failures: 0,
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
		Phase:    AccessPhase,
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
		cb := getOrCreateCircuitBreaker(serviceName)

		failureThreshold, successThreshold, timeout := plugin.getConfig()

		cb.mutex.Lock()

		// Check if we should transition from Open to Half-Open
		if cb.state == CircuitOpen {
			if time.Since(cb.lastFailureTime) > timeout {
				slog.Info("Circuit breaker transitioning to half-open", "service", serviceName)
				cb.state = CircuitHalfOpen
				cb.successes = 0
			}
		}

		// If circuit is open, reject immediately
		if cb.state == CircuitOpen {
			cb.mutex.Unlock()
			slog.Warn("Circuit breaker OPEN - rejecting request", "service", serviceName)
			model.NewHttpError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE",
				"Service temporarily unavailable (circuit breaker open)").WriteJSONResponse(w)
			return
		}

		cb.mutex.Unlock()

		// Create a response recorder to capture the response status
		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Execute the next handler
		next.ServeHTTP(recorder, r)

		// Update circuit breaker state based on response
		cb.mutex.Lock()
		defer cb.mutex.Unlock()

		if isFailureStatus(recorder.statusCode) {
			cb.failures++
			cb.lastFailureTime = time.Now()
			cb.successes = 0

			slog.Debug("Circuit breaker recorded failure",
				"service", serviceName,
				"failures", cb.failures,
				"threshold", failureThreshold)

			if cb.failures >= failureThreshold {
				slog.Warn("Circuit breaker OPENING", "service", serviceName, "failures", cb.failures)
				cb.state = CircuitOpen
			}
		} else {
			// Success
			if cb.state == CircuitHalfOpen {
				cb.successes++
				slog.Debug("Circuit breaker recorded success in half-open",
					"service", serviceName,
					"successes", cb.successes,
					"threshold", successThreshold)

				if cb.successes >= successThreshold {
					slog.Info("Circuit breaker CLOSING - service recovered", "service", serviceName)
					cb.state = CircuitClosed
					cb.failures = 0
					cb.successes = 0
				}
			} else {
				// Reset failures on success in closed state
				cb.failures = 0
			}
		}
	})
}

// responseRecorder wraps ResponseWriter to capture status code
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
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
