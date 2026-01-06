package gateway

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetryHandle_Basic(t *testing.T) {
	rh := NewRetryHandle("test-service")

	assert.Equal(t, "test-service", rh.ServiceName)
	assert.Equal(t, 0, rh.RetryCount)
	assert.Empty(t, rh.FailedUpstreams)
}

func TestRetryHandle_MarkFailed(t *testing.T) {
	rh := NewRetryHandle("test-service")

	// Mark an upstream as failed
	rh.MarkFailed("upstream-1")
	assert.True(t, rh.IsFailed("upstream-1"))
	assert.False(t, rh.IsFailed("upstream-2"))
	assert.Equal(t, "upstream-1", rh.LastUpstreamId)

	// Mark another as failed
	rh.MarkFailed("upstream-2")
	assert.True(t, rh.IsFailed("upstream-2"))
	assert.Len(t, rh.FailedUpstreams, 2)
}

func TestRetryHandle_IncrementRetry(t *testing.T) {
	rh := NewRetryHandle("test-service")

	assert.Equal(t, 0, rh.RetryCount)

	rh.IncrementRetry()
	assert.Equal(t, 1, rh.RetryCount)

	rh.IncrementRetry()
	assert.Equal(t, 2, rh.RetryCount)
}

func TestRetryHandle_GetFailedUpstreams(t *testing.T) {
	rh := NewRetryHandle("test-service")
	rh.MarkFailed("u1")
	rh.MarkFailed("u2")
	rh.MarkFailed("u3")

	failed := rh.GetFailedUpstreams()
	assert.Len(t, failed, 3)
	assert.True(t, failed["u1"])
	assert.True(t, failed["u2"])
	assert.True(t, failed["u3"])

	// Verify it's a copy (modifying doesn't affect original)
	delete(failed, "u1")
	assert.True(t, rh.IsFailed("u1"), "Original should not be affected")
}

func TestRetryHandle_HasAvailableUpstreams(t *testing.T) {
	rh := NewRetryHandle("test-service")
	totalUpstreams := 3

	assert.True(t, rh.HasAvailableUpstreams(totalUpstreams))

	rh.MarkFailed("u1")
	assert.True(t, rh.HasAvailableUpstreams(totalUpstreams))

	rh.MarkFailed("u2")
	assert.True(t, rh.HasAvailableUpstreams(totalUpstreams))

	rh.MarkFailed("u3")
	assert.False(t, rh.HasAvailableUpstreams(totalUpstreams), "Should return false when all failed")
}

func TestRetryHandle_Reset(t *testing.T) {
	rh := NewRetryHandle("test-service")
	rh.MarkFailed("u1")
	rh.MarkFailed("u2")
	rh.IncrementRetry()
	rh.IncrementRetry()

	assert.Len(t, rh.FailedUpstreams, 2)
	assert.Equal(t, 2, rh.RetryCount)

	rh.Reset()

	assert.Empty(t, rh.FailedUpstreams)
	assert.Equal(t, 0, rh.RetryCount)
	assert.Empty(t, rh.LastUpstreamId)
}

func TestRetryHandle_ContextIntegration(t *testing.T) {
	ctx := context.Background()

	// Initially no handle
	handle := GetRetryHandleFromContext(ctx)
	assert.Nil(t, handle)

	// Create and attach
	rh := NewRetryHandle("test-service")
	rh.MarkFailed("u1")
	ctx = WithRetryHandle(ctx, rh)

	// Retrieve
	retrieved := GetRetryHandleFromContext(ctx)
	assert.NotNil(t, retrieved)
	assert.True(t, retrieved.IsFailed("u1"))
}

func TestRetryHandle_GetOrCreateRetryHandle(t *testing.T) {
	ctx := context.Background()

	// First call creates new
	rh1, ctx := GetOrCreateRetryHandle(ctx, "svc1")
	assert.NotNil(t, rh1)
	assert.Equal(t, "svc1", rh1.ServiceName)

	rh1.MarkFailed("u1")

	// Second call returns existing
	rh2, _ := GetOrCreateRetryHandle(ctx, "svc2") // service name ignored if exists
	assert.Same(t, rh1, rh2)
	assert.True(t, rh2.IsFailed("u1"))
}

func TestRetryHandle_Concurrent(t *testing.T) {
	rh := NewRetryHandle("test-service")

	// Concurrent access
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			rh.MarkFailed(string(rune('0' + id)))
			rh.IsFailed(string(rune('0' + id)))
			rh.GetFailedUpstreams()
			rh.IncrementRetry()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, rh.RetryCount)
	assert.Len(t, rh.FailedUpstreams, 10)
}
