package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConnectionTracker(t *testing.T) {
	// Create a local instance for the test to avoid global state interference
	ct := &ConnectionTracker{}

	// Create a handler that takes some time
	handler := ct.Track(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	// 1. Start a slow request in the background
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.ServeHTTP(rr, req)
	}()

	// Give the goroutine a moment to start and be tracked
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, int64(1), ct.GetActiveCount(), "Should have 1 active request")

	// 2. Enter Draining mode
	ct.StartDraining()
	assert.True(t, ct.isDraining.Load(), "Should be in draining state")

	// 3. New request should be rejected immediately with 503
	req2 := httptest.NewRequest("GET", "/new", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusServiceUnavailable, rr2.Code, "New requests should get 503")

	// 4. Wait for the active request to complete
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := ct.Wait(ctx)
	assert.NoError(t, err, "Wait should complete without timeout")
	assert.Equal(t, int64(0), ct.GetActiveCount(), "Should have 0 active requests after Wait")

	wg.Wait()
	assert.Equal(t, http.StatusOK, rr.Code, "The original request should have completed successfully")
}

func TestConnectionTracker_Timeout(t *testing.T) {
	ct := &ConnectionTracker{}

	// Track a request that never finishes in the test duration
	ct.wg.Add(1)
	ct.active.Add(1)

	ct.StartDraining()

	// Wait with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := ct.Wait(ctx)
	assert.Error(t, err, "Wait should return context deadline exceeded")
	assert.Equal(t, context.DeadlineExceeded, err)
}
