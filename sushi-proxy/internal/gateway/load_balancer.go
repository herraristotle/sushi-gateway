package gateway

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"math"
	mathrand "math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/sony/gobreaker"
)

const (
	// SlowStartDuration is the time window over which a new upstream's weight ramps up
	SlowStartDuration = 60 * time.Second
)

// Contains all logic related to getting the upstream for load balancing based on the load balancing strategy.
type LoadBalancer struct {
	healthChecker *HealthChecker
}

// Create a round robin cache based on service name
// Stores the counter of upstream to map to
var roundRobinCache sync.Map

// Create a consistent hash cache based on service name
// Stores the consistent hash ring for each service
var consistentHashCache sync.Map

// Create a weighted round robin cache based on service name
// Stores the state of weighted round robin for each service
// Map[string]map[string]*WeightedState
var weightedCache sync.Map

// Global cache for active connections
// Map[string]map[string]int64 (ServiceName -> UpstreamID -> Count)
// Use sync.Map to store *int64 for atomic operations or a struct with Mutex
// Given we need updates, a Map of Maps is complex with pure sync.Map.
// Let's use a sync.Map where key=ServiceName, value=*ServiceConnectionState
var activeConnectionCache sync.Map

type ServiceConnectionState struct {
	Counts map[string]int64
	Lock   sync.Mutex
}

type WeightedState struct {
	CurrentWeight   int
	EffectiveWeight int
	FirstSeenAt     time.Time
}

type WeightedServiceState struct {
	States map[string]*WeightedState
	Lock   sync.Mutex
}

func NewLoadBalancer(healthChecker *HealthChecker) *LoadBalancer {
	return &LoadBalancer{healthChecker: healthChecker}
}

// isUpstreamAvailable checks if an upstream is healthy and not short-circuited by its circuit breaker
func (lb *LoadBalancer) isUpstreamAvailable(service model.Service, u model.UpstreamTarget) bool {
	// Always check Target Circuit Breaker first
	cb := GlobalTargetCBManager.GetBreaker(u.Target, &service, u.Id)
	cbState := cb.State()

	if cbState == gobreaker.StateOpen {
		return false
	}

	if service.Health.Enabled {
		if state, exists := lb.healthChecker.serviceHealthMap[service.Name][u.Id]; exists {
			if state.Status != Healthy {
				// If marked unhealthy but CB is HALF-OPEN, we must allow it through to probe for recovery
				if cbState == gobreaker.StateHalfOpen {
					slog.Info("Allowing probe to HALF-OPEN upstream", "service", service.Name, "upstream", u.Id)
					return true
				}
				slog.Debug("Upstream unhealthy in map", "service", service.Name, "upstream", u.Id, "cbState", cbState.String())
				return false
			}
		} else {
			// If not yet checked by health checker, assume unhealthy for safety
			return false
		}
	}

	return true
}

// Gets the index of upstream to forward the request to based on the load balancing algorithm
func (lb *LoadBalancer) GetNextUpstream(service model.Service, clientIP string, tags []string) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}
	switch service.LoadBalancingStrategy {
	case model.RoundRobin:
		return lb.handleRoundRobin(service, tags)
	case model.Weighted:
		return lb.handleWeighted(service, tags)
	case model.IPHash, model.ConsistentHashing:
		return lb.handleIPHash(service, clientIP, tags)
	case model.LeastConnections:
		return lb.handleLeastConnections(service, tags)
	case model.Latency:
		return lb.handleLatency(service, tags)
	default:
		return lb.handleRoundRobin(service, tags)
	}
}

// GetNextUpstreamWithRequest is the enhanced version that extracts hash values from
// the full HTTP request based on upstream configuration (hash_on, hash_on_header, etc.)
// This enables session stickiness on headers, cookies, paths, and other request attributes.
// Returns upstream index and optional cookie to set (if sticky session is new)
func (lb *LoadBalancer) GetNextUpstreamWithRequest(service model.Service, req *http.Request, upstreamConfig *model.UpstreamConfig, tags []string) (int, *http.Cookie) {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable, nil
	}

	switch service.LoadBalancingStrategy {
	case model.RoundRobin:
		return lb.handleRoundRobin(service, tags), nil
	case model.Weighted:
		return lb.handleWeighted(service, tags), nil
	case model.IPHash, model.ConsistentHashing:
		// Extract hash value based on upstream configuration
		hashValue, ok := ExtractHashValue(req, upstreamConfig)

		// Sticky Session Cookie Injection
		var cookieToSet *http.Cookie
		if (!ok || hashValue == "") && upstreamConfig != nil && upstreamConfig.HashOn == model.HashOnCookie {
			// Generate new session ID
			newSessionID := generateSessionID()
			hashValue = newSessionID

			// Return cookie to set
			cookieToSet = &http.Cookie{
				Name:     upstreamConfig.HashOnCookie,
				Value:    newSessionID,
				Path:     "/", // Scope to root for now
				HttpOnly: true,
			}
			if upstreamConfig.HashOnCookiePath != "" {
				cookieToSet.Path = upstreamConfig.HashOnCookiePath
			}
		}

		return lb.handleIPHash(service, hashValue, tags), cookieToSet
	case model.LeastConnections:
		return lb.handleLeastConnections(service, tags), nil
	case model.Latency:
		return lb.handleLatency(service, tags), nil
	default:
		return lb.handleRoundRobin(service, tags), nil
	}
}

func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// GetNextUpstreamWithRetry selects the next upstream while excluding failed ones
// This implements Kong's failedAddresses pattern for intelligent retry routing.
//
// Kong reference: kong/runloop/balancer/latency.lua (lines 187-195)
// - Filters out addresses that have already failed for this request
// - Falls back to including all addresses if all have failed
func (lb *LoadBalancer) GetNextUpstreamWithRetry(service model.Service, clientIP string, failedUpstreams map[string]bool, tags []string) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}

	// If no failed upstreams, use normal selection
	if len(failedUpstreams) == 0 {
		return lb.GetNextUpstream(service, clientIP, tags)
	}

	// If all upstreams have failed, reset and try again (Kong behavior)
	availableCount := 0
	for _, u := range service.Upstreams {
		if !failedUpstreams[u.Id] {
			availableCount++
		}
	}
	if availableCount == 0 {
		// All failed, use normal selection as fallback
		return lb.GetNextUpstream(service, clientIP, tags)
	}

	// Select from non-failed upstreams based on algorithm
	// Select from non-failed upstreams based on algorithm
	switch service.LoadBalancingStrategy {
	case model.LeastConnections:
		return lb.handleLeastConnectionsExcluding(service, failedUpstreams, tags)
	case model.Latency:
		return lb.handleLatencyExcluding(service, failedUpstreams, tags)
	default:
		// For other algorithms, filter and select best available
		return lb.handleRoundRobinExcluding(service, failedUpstreams, tags)
	}
}

// handleRoundRobinExcluding selects next upstream excluding failed ones
func (lb *LoadBalancer) handleRoundRobinExcluding(service model.Service, failedUpstreams map[string]bool, tags []string) int {
	// Get available indices
	var availableIndices []int
	for i, u := range service.Upstreams {
		if !failedUpstreams[u.Id] {
			// Check tags
			if !tagsMatch(u.Tags, tags) {
				continue
			}

			// Check availability (Health + Circuit Breaker)
			if !lb.isUpstreamAvailable(service, u) {
				continue
			}
			availableIndices = append(availableIndices, i)
		}
	}

	if len(availableIndices) == 0 {
		return model.NoUpstreamsAvailable
	}

	// Simple round-robin among available
	val, _ := roundRobinCache.LoadOrStore(service.Name+"_retry", 0)
	currentIndex := val.(int)

	// Find next available index
	selected := availableIndices[currentIndex%len(availableIndices)]
	roundRobinCache.Store(service.Name+"_retry", (currentIndex+1)%len(availableIndices))

	return selected
}

// handleLeastConnectionsExcluding uses heap to find best upstrea excluding failed ones
func (lb *LoadBalancer) handleLeastConnectionsExcluding(service model.Service, failedUpstreams map[string]bool, tags []string) int {
	// Combine failed upstreams with unhealthy ones
	excluded := make(map[string]bool)
	for id := range failedUpstreams {
		excluded[id] = true
	}

	if service.Health.Enabled {
		for _, u := range service.Upstreams {
			if state, exists := lb.healthChecker.serviceHealthMap[service.Name][u.Id]; exists {
				if state.Status != Healthy {
					excluded[u.Id] = true
				}
			}
		}
	}

	// Use binary heap with exclusions
	h := GetOrCreateLeastConnHeap(service)
	return h.GetBestUpstreamExcluding(excluded)
}

// Helper to check if tags match (Subset LB)
// Returns true if target has ALL tags in requiredTags
func tagsMatch(targetTags []string, requiredTags []string) bool {
	if len(requiredTags) == 0 {
		return true
	}
	if len(targetTags) == 0 {
		return false
	}

	// Check if all required tags are present in target tags
	for _, required := range requiredTags {
		found := false
		for _, target := range targetTags {
			if target == required {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// handleLatencyExcluding selects best latency upstream excluding failed ones
func (lb *LoadBalancer) handleLatencyExcluding(service model.Service, failedUpstreams map[string]bool, tags []string) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}

	// Get available upstream indices (healthy and not failed)
	var candidateIndices []int
	for i, u := range service.Upstreams {
		if failedUpstreams[u.Id] {
			continue
		}
		if !tagsMatch(u.Tags, tags) {
			continue
		}
		// Check availability (Health + Circuit Breaker)
		if !lb.isUpstreamAvailable(service, u) {
			continue
		}
		candidateIndices = append(candidateIndices, i)
	}

	if len(candidateIndices) == 0 {
		return model.NoUpstreamsAvailable
	}

	if len(candidateIndices) == 1 {
		return candidateIndices[0]
	}

	// Find lowest EWMA score among available
	var bestIdx int
	var lowestScore float64 = math.MaxFloat64

	for _, idx := range candidateIndices {
		u := service.Upstreams[idx]
		ewma := GetEWMA(service.Name, u.Id)
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

// Get the current upstream request is routed to.
func (lb *LoadBalancer) GetCurrentUpstream(service model.Service, clientIP string) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}
	switch service.LoadBalancingStrategy {
	case model.RoundRobin:
		if val, ok := roundRobinCache.Load(service.Name); ok {
			return val.(int)
		}
		return 0
	case model.IPHash:
		return lb.handleIPHash(service, clientIP, nil)
	default:
		return 0
	}
}

// ResetLoadBalancers the load balancer caches
func ResetLoadBalancers() {
	roundRobinCache = sync.Map{}
	consistentHashCache = sync.Map{}
	weightedCache = sync.Map{}
	activeConnectionCache = sync.Map{}
	ResetLatencyCache()
}

func IncrementActiveConnections(serviceName, upstreamId string) {
	val, _ := activeConnectionCache.LoadOrStore(serviceName, &ServiceConnectionState{
		Counts: make(map[string]int64),
	})
	state := val.(*ServiceConnectionState)
	state.Lock.Lock()
	defer state.Lock.Unlock()
	state.Counts[upstreamId]++
}

func DecrementActiveConnections(serviceName, upstreamId string) {
	val, ok := activeConnectionCache.Load(serviceName)
	if !ok {
		return
	}
	state := val.(*ServiceConnectionState)
	state.Lock.Lock()
	defer state.Lock.Unlock()
	if state.Counts[upstreamId] > 0 {
		state.Counts[upstreamId]--
	}
}

func GetActiveConnections(serviceName, upstreamId string) int64 {
	val, ok := activeConnectionCache.Load(serviceName)
	if !ok {
		return 0
	}
	state := val.(*ServiceConnectionState)
	state.Lock.Lock()
	defer state.Lock.Unlock()
	return state.Counts[upstreamId]
}

func (lb *LoadBalancer) handleLeastConnections(service model.Service, tags []string) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}
	if len(service.Upstreams) == 1 {
		return 0
	}

	// Use Power of Two Choices (P2C) for better concurrency and distribution
	// Instead of global locking with a heap, we pick 2 random nodes and choose the better one.
	return lb.handleLeastConnectionsP2C(service, tags)
}

// handleLeastConnectionsP2C uses Power of Two Choices algorithm
// O(1) complexity, no global lock contention (aside from ReadLocks in helpers)
func (lb *LoadBalancer) handleLeastConnectionsP2C(service model.Service, tags []string) int {
	candidates := lb.getHealthyCandidates(service, tags)
	numCandidates := len(candidates)

	if numCandidates == 0 {
		return model.NoUpstreamsAvailable
	}
	if numCandidates == 1 {
		return candidates[0]
	}

	// Pick two random indices
	// Use global rand for simplicity, seed should be init in main
	idx1 := mathrand.Intn(numCandidates)
	idx2 := mathrand.Intn(numCandidates)
	// Ensure they are different if possible (only loops if num > 1, which it is)
	for idx1 == idx2 {
		idx2 = mathrand.Intn(numCandidates)
	}

	u1 := service.Upstreams[candidates[idx1]]
	u2 := service.Upstreams[candidates[idx2]]

	score1 := lb.calculateLeastConnScore(service.Name, u1)
	score2 := lb.calculateLeastConnScore(service.Name, u2)

	if score1 < score2 {
		return candidates[idx1]
	}
	return candidates[idx2]
}

func (lb *LoadBalancer) calculateLeastConnScore(serviceName string, u model.UpstreamTarget) float64 {
	conns := GetActiveConnections(serviceName, u.Id)
	weight := float64(u.Weight)
	if weight <= 0 {
		weight = 1
	}
	// (active + 1) / weight
	return float64(conns+1) / weight
}

func (lb *LoadBalancer) getHealthyCandidates(service model.Service, tags []string) []int {
	var candidates []int
	for i, u := range service.Upstreams {
		if !lb.isUpstreamAvailable(service, u) {
			continue
		}
		candidates = append(candidates, i)
	}
	// Fallback if all unhealthy? Kong does "all failed -> try all".
	// But "all unhealthy" usually means return 503.
	// The existing logic returned NoUpstreamsAvailable.
	return candidates
}

// handleLeastConnectionsHeap uses binary heap for O(log N) selection
// Used for large upstream pools (>= 10 upstreams)
func (lb *LoadBalancer) handleLeastConnectionsHeap(service model.Service) int {
	h := GetOrCreateLeastConnHeap(service)

	if service.Health.Enabled {
		// Build set of unhealthy upstreams to exclude
		unhealthy := make(map[string]bool)
		for _, u := range service.Upstreams {
			if !lb.isUpstreamAvailable(service, u) {
				unhealthy[u.Id] = true
			}
		}

		if len(unhealthy) == len(service.Upstreams) {
			return model.NoUpstreamsAvailable
		}

		return h.GetBestUpstreamExcluding(unhealthy)
	}

	return h.GetBestUpstream()
}

// handleLeastConnectionsLinear uses O(N) linear scan for small pools
func (lb *LoadBalancer) handleLeastConnectionsLinear(service model.Service) int {
	// Filter healthy upstreams
	var candidateIndices []int
	if service.Health.Enabled {
		for i, u := range service.Upstreams {
			if lb.isUpstreamAvailable(service, u) {
				candidateIndices = append(candidateIndices, i)
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

	// Weight-aware least-connections using Kong's formula:
	// score = (connections + 1) / weight
	// The +1 ensures that even with 0 connections, higher weight wins
	var bestIdx int
	var lowestScore float64 = math.MaxFloat64

	for _, idx := range candidateIndices {
		u := service.Upstreams[idx]
		conns := GetActiveConnections(service.Name, u.Id)

		// Calculate weight (default to 1 if not set)
		weight := float64(u.Weight)
		if weight <= 0 {
			weight = 1
		}

		// Kong formula: (connections + 1) / weight
		score := float64(conns+1) / weight

		if score < lowestScore {
			lowestScore = score
			bestIdx = idx
		}
	}

	return bestIdx
}

func (lb *LoadBalancer) handleIPHash(service model.Service, clientIP string, tags []string) int {
	// 1. Ketama compatibility check
	// If algorithm is explicitly set to ketama mechanism (via upstream config or checking algo name?)
	// The Service struct has LoadBalancingStrategy enum. We might need a new enum value or check upstream config.
	// For now, let's assume if it falls into this case (ConsistentHashing) we check specific implementations.

	// Get or create consistent hash ring for this service
	// We might need distinct caching for Ketama vs Maglev if they can switch dynamically?
	// For simplicity, we assume one ring type per service lifetime.

	ring, _ := consistentHashCache.LoadOrStore(service.Name, NewConsistentHashRing(service))

	// Check for Bounded Load Configuration (e.g. HashBalanceFactor)
	// Default to 1.25 (125%) if not specified, but only ENABLE if we have active connection tracking.
	// Let's hardcode Factor=1.25 for this enhancement as per plan.
	maxLoadFactor := 1.25

	// Calculate average load
	// We need total active connections for this service.
	var totalConns int64
	var healthyCount int

	// Iterate to sum connections
	for _, u := range service.Upstreams {
		conns := GetActiveConnections(service.Name, u.Id)
		totalConns += conns

		// Check health
		healthy := lb.isUpstreamAvailable(service, u)
		if healthy {
			healthyCount++
		}
	}

	averageLoad := 0.0
	if healthyCount > 0 {
		averageLoad = float64(totalConns) / float64(healthyCount)
	}

	// Max allowed load per host
	maxLoad := averageLoad * maxLoadFactor
	// Ensure a minimum floor to avoid overly sensitive skipping on low traffic
	if maxLoad < 1.0 {
		maxLoad = 1.0
	}

	filter := func(target model.UpstreamTarget) bool {
		// 1. Tag Check (Subset LB)
		if !tagsMatch(target.Tags, tags) {
			return false
		}

		// 2. Health & CB Check
		if !lb.isUpstreamAvailable(service, target) {
			return false
		}

		// 2. Health Check
		if service.Health.Enabled {
			if state, exists := lb.healthChecker.serviceHealthMap[service.Name][target.Id]; exists {
				if state.Status != Healthy {
					return false
				}
			}
		}

		// 3. Bounded Load Check
		// Only check if we have enough traffic to matter (e.g. at least 10 active connections globally)
		if totalConns > 10 {
			active := GetActiveConnections(service.Name, target.Id)
			if float64(active) > maxLoad {
				// Reject this target, Maglev will probe next
				return false
			}
		}

		return true
	}

	var upstream model.UpstreamTarget

	if maglevRing, ok := ring.(*ConsistentHashRing); ok {
		upstream = maglevRing.GetUpstreamWithFilter(clientIP, filter)
	} else if ketamaRing, ok := ring.(*KetamaRing); ok {
		// Ketama doesn't support Bounded Load probing easily yet, just return
		upstream = ketamaRing.GetUpstream(clientIP)
	} else {
		// Fallback re-type assertion or recreation
		// In case type mismatch from cache, force new Maglev
		newLimit := NewConsistentHashRing(service)
		consistentHashCache.Store(service.Name, newLimit)
		upstream = newLimit.GetUpstreamWithFilter(clientIP, filter)
	}

	// Final verification of returned upstream (maglev might have failed open to primary)
	if upstream.Id != "" {
		if tagsMatch(upstream.Tags, tags) && lb.isUpstreamAvailable(service, upstream) {
			// Found healthy, tagged upstream
			for i, u := range service.Upstreams {
				if u.Id == upstream.Id {
					return i
				}
			}
		}

		// Fallback for case where health check is disabled but tags match
		if !service.Health.Enabled && tagsMatch(upstream.Tags, tags) {
			for i, u := range service.Upstreams {
				if u.Id == upstream.Id {
					return i
				}
			}
		}
	}

	return model.NoUpstreamsAvailable
}

func (lb *LoadBalancer) handleRoundRobin(service model.Service, tags []string) int {

	// TODO: probably refactor this code, it's a bit messy
	// Health check is not enabled, so we just round robin through all upstreams
	if !service.Health.Enabled {
		if len(service.Upstreams) == 1 {
			// Check tags
			if tagsMatch(service.Upstreams[0].Tags, tags) {
				return 0
			}
			return model.NoUpstreamsAvailable
		}

		// Filter upstreams based on tags
		// Round robin logic needs to skip unmatched.
		// Optimized approach: find valid indices then RR on them.

		var validIndices []int
		for i, u := range service.Upstreams {
			if tagsMatch(u.Tags, tags) {
				validIndices = append(validIndices, i)
			}
		}

		if len(validIndices) == 0 {
			return model.NoUpstreamsAvailable
		}

		currentVal, _ := roundRobinCache.LoadOrStore(service.Name, 0)
		currentIndex := currentVal.(int)

		// Find index in validIndices
		// Simple RR on validIndices

		// If we store global index, we just incr and wrap.
		// index = (global % len(valid))
		// chosen = valid[index]

		nextVal := (currentIndex + 1) % len(validIndices)
		roundRobinCache.Store(service.Name, nextVal)

		return validIndices[currentIndex%len(validIndices)]
	}

	// Health check is enabled, so we need to get the healthy upstreams to round robin through
	healthyUpstreams := lb.healthChecker.GetHealthyUpstreams(service)
	// Filter healthy upstreams by tags
	var filteredHealthy []model.UpstreamTarget
	for _, u := range healthyUpstreams {
		if tagsMatch(u.Tags, tags) {
			filteredHealthy = append(filteredHealthy, u)
		}
	}

	if len(filteredHealthy) == 0 {
		return model.NoUpstreamsAvailable
	}

	if len(service.Upstreams) == 1 {
		return 0
	}

	// Get or initialize the current index
	currentVal, _ := roundRobinCache.LoadOrStore(service.Name, 0)
	currentIndex := currentVal.(int)

	numUpstreams := len(service.Upstreams)
	// Try to find the next healthy upstream
	for i := 0; i < numUpstreams; i++ {
		// Calculate next index with wraparound
		candidateIndex := (currentIndex + i) % numUpstreams
		upstream := service.Upstreams[candidateIndex]

		// Check tags
		if !tagsMatch(upstream.Tags, tags) {
			continue
		}

		// Check availability (Health + Circuit Breaker)
		if lb.isUpstreamAvailable(service, upstream) {
			// Store the next index for subsequent requests
			nextIndex := (candidateIndex + 1) % numUpstreams
			roundRobinCache.Store(service.Name, nextIndex)
			return candidateIndex
		}
	}

	// No healthy upstreams found,
	return model.NoUpstreamsAvailable
}

func (lb *LoadBalancer) handleWeighted(service model.Service, tags []string) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}
	if len(service.Upstreams) == 1 {
		return 0
	}

	// Filter healthy upstreams first if health check is enabled
	var candidateIndices []int
	for i, u := range service.Upstreams {
		if !tagsMatch(u.Tags, tags) {
			continue // Skip if tags don't match
		}

		if lb.isUpstreamAvailable(service, u) {
			candidateIndices = append(candidateIndices, i)
		}
	}

	if len(candidateIndices) == 0 {
		return model.NoUpstreamsAvailable
	}

	if len(candidateIndices) == 1 {
		return candidateIndices[0]
	}

	wrapperVal, _ := weightedCache.LoadOrStore(service.Name, &WeightedServiceState{
		States: make(map[string]*WeightedState),
	})
	wrapper := wrapperVal.(*WeightedServiceState)

	wrapper.Lock.Lock()
	defer wrapper.Lock.Unlock()

	totalWeight := 0
	var bestUpstreamIndex int = -1
	maxCurrentWeight := -1 << 31

	// Run SWRR on candidates
	now := time.Now()
	for _, index := range candidateIndices {
		u := service.Upstreams[index]

		state, exists := wrapper.States[u.Id]
		if !exists {
			weight := u.Weight
			if weight <= 0 {
				weight = 1 // Default to 1 to ensure it gets some traffic
			}
			state = &WeightedState{
				CurrentWeight:   0,
				EffectiveWeight: weight,
				FirstSeenAt:     now,
			}
			wrapper.States[u.Id] = state
		}

		// Calculate configured weight
		configuredWeight := u.Weight
		if configuredWeight <= 0 {
			configuredWeight = 1
		}

		// Apply Slow Start Logic
		// If uptime < SlowStartDuration, scale weight linearly
		uptime := now.Sub(state.FirstSeenAt)
		effectiveWeight := configuredWeight

		if uptime < SlowStartDuration {
			// Linear ramp up: weight * (uptime / duration)
			// Ensure at least 1
			factor := float64(uptime) / float64(SlowStartDuration)
			if factor < 0.1 {
				factor = 0.1
			} // Minimum start validity
			effectiveWeight = int(float64(configuredWeight) * factor)
			if effectiveWeight < 1 {
				effectiveWeight = 1
			}
		}
		state.EffectiveWeight = effectiveWeight

		state.CurrentWeight += state.EffectiveWeight
		totalWeight += state.EffectiveWeight

		if state.CurrentWeight > maxCurrentWeight {
			maxCurrentWeight = state.CurrentWeight
			bestUpstreamIndex = index
		}
	}

	if bestUpstreamIndex != -1 {
		bestUpstream := service.Upstreams[bestUpstreamIndex]
		if state, ok := wrapper.States[bestUpstream.Id]; ok {
			state.CurrentWeight -= totalWeight
		}
		return bestUpstreamIndex
	}

	return 0 // Fallback
}
