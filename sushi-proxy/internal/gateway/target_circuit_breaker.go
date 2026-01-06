package gateway

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/sony/gobreaker"
)

// TargetCBManager manages circuit breakers for individual upstream targets
type TargetCBManager struct {
	breakers sync.Map
}

var (
	GlobalTargetCBManager = &TargetCBManager{}
)

// GetBreaker retrieves or creates a CB for the given target address, utilizing service configuration
func (m *TargetCBManager) GetBreaker(targetAddr string, service *model.Service, upstreamId string) *gobreaker.CircuitBreaker {
	// Key remains targetAddr for pool sharing, but we could use service+upstream if isolation is preferred.
	// Kong's circuit breaker is usually per-upstream.
	key := fmt.Sprintf("%s_%s_%s", service.Name, upstreamId, targetAddr)

	if cb, ok := m.breakers.Load(key); ok {
		return cb.(*gobreaker.CircuitBreaker)
	}

	// Default settings
	maxRequests := uint32(5)
	interval := 60 * time.Second
	timeout := 10 * time.Second
	failureThreshold := 10
	failureRate := 0.5

	// Map Passive Health Check configuration if available
	if service != nil && service.UpstreamHealthChecks != nil && service.UpstreamHealthChecks.Passive != nil {
		passive := service.UpstreamHealthChecks.Passive
		if passive.Unhealthy != nil && passive.Unhealthy.HttpFailures > 0 {
			failureThreshold = passive.Unhealthy.HttpFailures
		}
		// In gobreaker, ReadyToTrip is the trigger. We can use failureThreshold as a simple count.
	}

	settings := gobreaker.Settings{
		Name:        key,
		MaxRequests: maxRequests,
		Interval:    interval,
		Timeout:     timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip if > threshold failures
			return counts.ConsecutiveFailures >= uint32(failureThreshold) ||
				(counts.Requests >= 10 && float64(counts.TotalFailures)/float64(counts.Requests) >= failureRate)
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			if to == gobreaker.StateOpen {
				GlobalHealthChecker.UpdateHealthStatus(service.Name, upstreamId, Unhealthy)
			} else if to == gobreaker.StateClosed {
				GlobalHealthChecker.UpdateHealthStatus(service.Name, upstreamId, Healthy)
			}
		},
	}

	cb := gobreaker.NewCircuitBreaker(settings)
	actual, _ := m.breakers.LoadOrStore(key, cb)
	return actual.(*gobreaker.CircuitBreaker)
}

// Allow checks if the target is available (Circuit Breaker is not Open)
func (m *TargetCBManager) Allow(targetAddr string, service *model.Service, upstreamId string) bool {
	return m.GetBreaker(targetAddr, service, upstreamId).State() != gobreaker.StateOpen
}

// CircuitBreakerTransport wraps http.RoundTripper with target-specific circuit breaking
type CircuitBreakerTransport struct {
	Target     string
	Service    *model.Service
	UpstreamID string
	Base       http.RoundTripper
}

func (t *CircuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cb := GlobalTargetCBManager.GetBreaker(t.Target, t.Service, t.UpstreamID)

	var resp *http.Response

	// Execute wraps the network call
	_, err := cb.Execute(func() (interface{}, error) {
		var err error
		resp, err = t.Base.RoundTrip(req)
		if err != nil {
			// Network error -> Failure
			return nil, err
		}

		// Check for 5xx errors to trip the breaker
		if resp.StatusCode >= 500 {
			// Close body immediately as we are treating this as a failure/retryable
			resp.Body.Close()
			return nil, fmt.Errorf("upstream 5xx error: %d", resp.StatusCode)
		}

		return resp, nil
	})

	if err != nil {
		// Circuit Breaker blocked request (ErrOpenState) or Network/5xx error
		// Return error to trigger load balancer retry logic
		return nil, err
	}

	// Success (2xx, 3xx, 4xx)
	return resp, nil
}
