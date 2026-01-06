package gateway

import (
	"context"
	"sync"
)

// RetryHandle tracks the state of a request across retries, following Kong's pattern.
// It stores failed upstream addresses so they won't be selected again during retry attempts.
//
// Kong reference: kong/runloop/balancer/latency.lua (lines 150-163)
// - handle.failedAddresses tracks which upstreams failed
// - handle.retryCount tracks number of retry attempts
// - On retry, failed addresses are filtered out from selection

// RetryHandle stores retry state for a single request
type RetryHandle struct {
	FailedUpstreams map[string]bool // Upstream IDs that failed and should be skipped
	RetryCount      int             // Number of retry attempts so far
	ServiceName     string          // Service being routed
	LastUpstreamId  string          // Most recently tried upstream
	mu              sync.RWMutex
}

// contextKey for storing retry handle in request context
type retryHandleKey struct{}

// NewRetryHandle creates a new retry handle for a request
func NewRetryHandle(serviceName string) *RetryHandle {
	return &RetryHandle{
		FailedUpstreams: make(map[string]bool),
		RetryCount:      0,
		ServiceName:     serviceName,
	}
}

// MarkFailed marks an upstream as failed so it won't be selected on retry
func (h *RetryHandle) MarkFailed(upstreamId string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.FailedUpstreams[upstreamId] = true
	h.LastUpstreamId = upstreamId
}

// IsFailed checks if an upstream has already failed
func (h *RetryHandle) IsFailed(upstreamId string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.FailedUpstreams[upstreamId]
}

// IncrementRetry increments the retry counter
func (h *RetryHandle) IncrementRetry() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.RetryCount++
}

// GetFailedUpstreams returns the set of failed upstream IDs
func (h *RetryHandle) GetFailedUpstreams() map[string]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	// Return a copy to avoid race conditions
	result := make(map[string]bool, len(h.FailedUpstreams))
	for k, v := range h.FailedUpstreams {
		result[k] = v
	}
	return result
}

// HasAvailableUpstreams checks if there are any upstreams not in the failed list
func (h *RetryHandle) HasAvailableUpstreams(totalUpstreams int) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.FailedUpstreams) < totalUpstreams
}

// Reset clears the failed upstream list (for testing)
func (h *RetryHandle) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.FailedUpstreams = make(map[string]bool)
	h.RetryCount = 0
	h.LastUpstreamId = ""
}

// --- Context integration ---

// WithRetryHandle adds a retry handle to the context
func WithRetryHandle(ctx context.Context, handle *RetryHandle) context.Context {
	return context.WithValue(ctx, retryHandleKey{}, handle)
}

// GetRetryHandleFromContext retrieves the retry handle from context
func GetRetryHandleFromContext(ctx context.Context) *RetryHandle {
	val := ctx.Value(retryHandleKey{})
	if val == nil {
		return nil
	}
	return val.(*RetryHandle)
}

// GetOrCreateRetryHandle gets existing handle or creates new one
func GetOrCreateRetryHandle(ctx context.Context, serviceName string) (*RetryHandle, context.Context) {
	handle := GetRetryHandleFromContext(ctx)
	if handle == nil {
		handle = NewRetryHandle(serviceName)
		ctx = WithRetryHandle(ctx, handle)
	}
	return handle, ctx
}
