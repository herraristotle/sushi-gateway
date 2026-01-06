package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
)

// ConnectionTracker tracks active requests to ensure graceful shutdown
type ConnectionTracker struct {
	wg         sync.WaitGroup
	isDraining atomic.Bool
	active     atomic.Int64
}

var GlobalConnectionTracker = &ConnectionTracker{}

// Track is a middleware that tracks active requests and rejets new ones during draining
func (ct *ConnectionTracker) Track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// If we are draining, reject new requests immediately
		if ct.isDraining.Load() {
			w.Header().Set("Connection", "close")
			w.Header().Set("Retry-After", "10")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Server is draining connections. Please retry on another instance."))
			return
		}

		// Increment tracked requests
		ct.wg.Add(1)
		ct.active.Add(1)
		defer func() {
			ct.wg.Done()
			ct.active.Add(-1)
		}()

		next.ServeHTTP(w, r)
	})
}

// StartDraining puts the tracker into draining mode
func (ct *ConnectionTracker) StartDraining() {
	if ct.isDraining.CompareAndSwap(false, true) {
		slog.Warn("Server entering DRAIN mode. No longer accepting new requests.")
	}
}

// Wait waits for all active requests to complete or until timeout
func (ct *ConnectionTracker) Wait(ctx context.Context) error {
	slog.Info("Waiting for active requests to finish...", "count", ct.active.Load())

	done := make(chan struct{})
	go func() {
		ct.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("All active requests completed successfully.")
		return nil
	case <-ctx.Done():
		slog.Error("Shutdown timeout reached while waiting for requests", "remaining", ct.active.Load())
		return ctx.Err()
	}
}

// GetActiveCount returns the current number of in-flight requests
func (ct *ConnectionTracker) GetActiveCount() int64 {
	return ct.active.Load()
}
