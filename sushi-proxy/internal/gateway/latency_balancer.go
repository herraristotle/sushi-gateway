package gateway

import (
	"math"
	"sync"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// Latency-based load balancer using Exponential Weighted Moving Average (EWMA)
// Inspired by Kong's latency balancer and Twitter's Finagle EWMA implementation

const (
	// DecayTime controls how quickly old latency measurements lose importance
	// Lower values = more responsive to recent changes, higher = more stable
	DecayTime = 10.0 // seconds

	// PickSetSize for Power of Two Choices algorithm
	// Randomly pick this many candidates and choose the one with lowest score
	PickSetSize = 2
)

// LatencyState tracks EWMA latency for a single upstream
type LatencyState struct {
	EWMA          float64   // Exponentially weighted moving average of response time
	LastTouchedAt time.Time // When this upstream was last measured
	mu            sync.RWMutex
}

// latencyCache stores EWMA state per service per upstream
// Key: "serviceName:upstreamId" -> *LatencyState
var latencyCache sync.Map

// decayEWMA calculates the decayed EWMA incorporating a new RTT measurement
func decayEWMA(currentEWMA float64, lastTouched time.Time, rtt float64, now time.Time) float64 {
	td := now.Sub(lastTouched).Seconds()
	if td < 0 {
		td = 0
	}
	// Exponential decay weight - older values contribute less
	weight := math.Exp(-td / DecayTime)
	return currentEWMA*weight + rtt*(1.0-weight)
}

// getLatencyState retrieves or creates the latency state for an upstream
func getLatencyState(serviceName, upstreamId string) *LatencyState {
	key := serviceName + ":" + upstreamId
	val, ok := latencyCache.Load(key)
	if ok {
		return val.(*LatencyState)
	}

	// Create new state with slow-start EWMA
	slowStart := calculateSlowStartEWMA(serviceName)
	state := &LatencyState{
		EWMA:          slowStart,
		LastTouchedAt: time.Now(),
	}
	actual, _ := latencyCache.LoadOrStore(key, state)
	return actual.(*LatencyState)
}

// calculateSlowStartEWMA returns the average EWMA of existing upstreams
// This prevents new upstreams from being overwhelmed with traffic
func calculateSlowStartEWMA(serviceName string) float64 {
	var total float64
	var count int

	prefix := serviceName + ":"
	latencyCache.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok && len(k) > len(prefix) && k[:len(prefix)] == prefix {
			state := value.(*LatencyState)
			state.mu.RLock()
			total += state.EWMA
			state.mu.RUnlock()
			count++
		}
		return true
	})

	if count == 0 {
		return 0 // No penalty for first upstream
	}
	return total / float64(count)
}

// GetEWMA returns the current EWMA score for an upstream (without updating)
func GetEWMA(serviceName, upstreamId string) float64 {
	state := getLatencyState(serviceName, upstreamId)
	state.mu.RLock()
	defer state.mu.RUnlock()

	now := time.Now()
	// Project the current EWMA forward in time (decay without new measurement)
	return decayEWMA(state.EWMA, state.LastTouchedAt, 0, now)
}

// RecordLatency updates the EWMA with a new response time measurement
// Call this after each request completes
func RecordLatency(serviceName, upstreamId string, duration time.Duration) {
	state := getLatencyState(serviceName, upstreamId)
	rtt := duration.Seconds()

	state.mu.Lock()
	defer state.mu.Unlock()

	now := time.Now()
	state.EWMA = decayEWMA(state.EWMA, state.LastTouchedAt, rtt, now)
	state.LastTouchedAt = now
}

// handleLatency implements the latency-based load balancing using Power of Two Choices
func (lb *LoadBalancer) handleLatency(service model.Service) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}
	if len(service.Upstreams) == 1 {
		return 0
	}

	// Get healthy upstream indices
	var candidateIndices []int
	if service.Health.Enabled {
		for i, u := range service.Upstreams {
			if state, exists := lb.healthChecker.serviceHealthMap[service.Name][u.Id]; exists {
				if state.Status == Healthy {
					candidateIndices = append(candidateIndices, i)
				}
			}
		}
		if len(candidateIndices) == 0 {
			return model.NoUpstreamsAvailable
		}
	} else {
		for i := range service.Upstreams {
			candidateIndices = append(candidateIndices, i)
		}
	}

	if len(candidateIndices) == 1 {
		return candidateIndices[0]
	}

	// Power of Two Choices: pick k random candidates, choose lowest score
	k := PickSetSize
	if len(candidateIndices) < k {
		k = len(candidateIndices)
	}

	// For simplicity, we'll check all candidates but weight the score
	// In production, you'd randomly sample k candidates
	var bestIdx int
	var lowestScore float64 = math.MaxFloat64

	for _, idx := range candidateIndices {
		u := service.Upstreams[idx]
		ewma := GetEWMA(service.Name, u.Id)
		// Score = EWMA / weight (lower is better)
		// Adding 1ms to avoid division by zero for zero-latency upstreams
		weight := float64(u.Weight)
		if weight <= 0 {
			weight = 1
		}
		score := (ewma + 0.001) / weight

		if score < lowestScore {
			lowestScore = score
			bestIdx = idx
		}
	}

	return bestIdx
}

// ResetLatencyCache clears all latency tracking state
func ResetLatencyCache() {
	latencyCache = sync.Map{}
}
