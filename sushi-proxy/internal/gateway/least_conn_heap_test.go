package gateway

import (
	"sync"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestLeastConnectionsHeap_Basic(t *testing.T) {
	// Reset for clean state
	ResetLeastConnHeapCache()
	activeConnectionCache = sync.Map{}

	upstreams := []model.UpstreamTarget{
		{Id: "u1", Target: "localhost:8081", Weight: 1},
		{Id: "u2", Target: "localhost:8082", Weight: 1},
		{Id: "u3", Target: "localhost:8083", Weight: 1},
	}

	h := NewLeastConnectionsHeap(upstreams, "test-service")

	// All have 0 connections, first one should be picked (stable sort)
	assert.Equal(t, 3, h.Len())

	best := h.GetBestUpstream()
	assert.True(t, best >= 0 && best < 3, "Should return valid index")
}

func TestLeastConnectionsHeap_SelectsLowestConnections(t *testing.T) {
	ResetLeastConnHeapCache()
	activeConnectionCache = sync.Map{}

	// Set up connection counts
	IncrementActiveConnections("test-heap-service", "u1")
	IncrementActiveConnections("test-heap-service", "u1")
	IncrementActiveConnections("test-heap-service", "u2")
	// u3 has 0 connections

	upstreams := []model.UpstreamTarget{
		{Id: "u1", Target: "localhost:8081", Weight: 1}, // 2 connections
		{Id: "u2", Target: "localhost:8082", Weight: 1}, // 1 connection
		{Id: "u3", Target: "localhost:8083", Weight: 1}, // 0 connections
	}

	h := NewLeastConnectionsHeap(upstreams, "test-heap-service")

	// u3 should be selected (0 connections)
	best := h.GetBestUpstream()
	assert.Equal(t, 2, best, "Should select u3 (index 2) with lowest connections")
}

func TestLeastConnectionsHeap_WeightAware(t *testing.T) {
	ResetLeastConnHeapCache()
	activeConnectionCache = sync.Map{}

	// u1: 2 connections, weight 4 -> score = 3/4 = 0.75
	// u2: 1 connection, weight 1 -> score = 2/1 = 2.0
	// u3: 0 connections, weight 1 -> score = 1/1 = 1.0
	IncrementActiveConnections("test-weight-service", "u1")
	IncrementActiveConnections("test-weight-service", "u1")
	IncrementActiveConnections("test-weight-service", "u2")

	upstreams := []model.UpstreamTarget{
		{Id: "u1", Target: "localhost:8081", Weight: 4}, // score 0.75
		{Id: "u2", Target: "localhost:8082", Weight: 1}, // score 2.0
		{Id: "u3", Target: "localhost:8083", Weight: 1}, // score 1.0
	}

	h := NewLeastConnectionsHeap(upstreams, "test-weight-service")

	// u1 should be selected (lowest score due to high weight)
	best := h.GetBestUpstream()
	assert.Equal(t, 0, best, "Should select u1 (index 0) with lowest score")
}

func TestLeastConnectionsHeap_UpdateConnection(t *testing.T) {
	ResetLeastConnHeapCache()
	activeConnectionCache = sync.Map{}

	upstreams := []model.UpstreamTarget{
		{Id: "u1", Target: "localhost:8081", Weight: 1},
		{Id: "u2", Target: "localhost:8082", Weight: 1},
	}

	h := NewLeastConnectionsHeap(upstreams, "test-update-service")

	// Initially both have same score, first one wins
	initial := h.GetBestUpstream()
	assert.True(t, initial == 0 || initial == 1)

	// Add connections to the first one
	if initial == 0 {
		h.UpdateConnection("u1", 5)
	} else {
		h.UpdateConnection("u2", 5)
	}

	// Now the other one should be selected
	afterUpdate := h.GetBestUpstream()
	assert.NotEqual(t, initial, afterUpdate, "Should select different upstream after adding connections")
}

func TestLeastConnectionsHeap_ExcludeFailedUpstreams(t *testing.T) {
	ResetLeastConnHeapCache()
	activeConnectionCache = sync.Map{}

	upstreams := []model.UpstreamTarget{
		{Id: "u1", Target: "localhost:8081", Weight: 1}, // index 0
		{Id: "u2", Target: "localhost:8082", Weight: 1}, // index 1
		{Id: "u3", Target: "localhost:8083", Weight: 1}, // index 2
	}

	h := NewLeastConnectionsHeap(upstreams, "test-exclude-service")

	// Get best without exclusions
	best := h.GetBestUpstream()
	bestId := upstreams[best].Id

	// Now exclude that best one (simulate failed retry)
	failed := map[string]bool{bestId: true}
	secondBest := h.GetBestUpstreamExcluding(failed)

	assert.NotEqual(t, best, secondBest, "Should select different upstream when best is excluded")
	assert.NotEqual(t, model.NoUpstreamsAvailable, secondBest)
}

func TestLeastConnectionsHeap_AllExcluded(t *testing.T) {
	ResetLeastConnHeapCache()
	activeConnectionCache = sync.Map{}

	upstreams := []model.UpstreamTarget{
		{Id: "u1", Target: "localhost:8081", Weight: 1},
		{Id: "u2", Target: "localhost:8082", Weight: 1},
	}

	h := NewLeastConnectionsHeap(upstreams, "test-all-excluded-service")

	// Exclude all upstreams
	failed := map[string]bool{"u1": true, "u2": true}
	result := h.GetBestUpstreamExcluding(failed)

	assert.Equal(t, model.NoUpstreamsAvailable, result, "Should return NoUpstreamsAvailable when all excluded")
}

func TestLeastConnectionsHeap_LargePool(t *testing.T) {
	ResetLeastConnHeapCache()
	activeConnectionCache = sync.Map{}

	// Create 20 upstreams (exceeds heap threshold of 10)
	upstreams := make([]model.UpstreamTarget, 20)
	for i := 0; i < 20; i++ {
		upstreams[i] = model.UpstreamTarget{
			Id:     string(rune('a' + i)),
			Target: "localhost:8080",
			Weight: 1,
		}
	}

	// Add connections to first 10 upstreams
	for i := 0; i < 10; i++ {
		for j := 0; j < i+1; j++ {
			IncrementActiveConnections("test-large-service", upstreams[i].Id)
		}
	}

	h := NewLeastConnectionsHeap(upstreams, "test-large-service")

	// Should select one of the last 10 upstreams (0 connections)
	best := h.GetBestUpstream()
	assert.True(t, best >= 10 && best < 20, "Should select upstream with 0 connections")
}
