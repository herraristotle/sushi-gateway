package gateway

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// TargetCBManager manages circuit breakers for individual upstream targets
type TargetCBManager struct {
	breakers sync.Map
}

var (
	GlobalTargetCBManager = &TargetCBManager{}
)

// GetBreaker retrieves or creates a CB for the given target address
func (m *TargetCBManager) GetBreaker(targetAddr string) *gobreaker.CircuitBreaker {
	if cb, ok := m.breakers.Load(targetAddr); ok {
		return cb.(*gobreaker.CircuitBreaker)
	}

	// Default settings
	// TODO: Make configurable via ProxyConfig if desired
	settings := gobreaker.Settings{
		Name:        targetAddr,
		MaxRequests: 5,                // Allow 5 concurrent requests in half-open state
		Interval:    60 * time.Second, // Interval to cycle counts
		Timeout:     10 * time.Second, // Duration of open state
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip if > 10 requests and > 50% failure rate
			return counts.Requests >= 10 && float64(counts.TotalFailures)/float64(counts.Requests) >= 0.5
		},
	}

	cb := gobreaker.NewCircuitBreaker(settings)
	actual, _ := m.breakers.LoadOrStore(targetAddr, cb)
	return actual.(*gobreaker.CircuitBreaker)
}

// Allow checks if the target is available (Circuit Breaker is not Open)
func (m *TargetCBManager) Allow(targetAddr string) bool {
	return m.GetBreaker(targetAddr).State() != gobreaker.StateOpen
}

// CircuitBreakerTransport wraps http.RoundTripper with target-specific circuit breaking
type CircuitBreakerTransport struct {
	Target string
	Base   http.RoundTripper
}

func (t *CircuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cb := GlobalTargetCBManager.GetBreaker(t.Target)

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
