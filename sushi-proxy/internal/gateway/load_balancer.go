package gateway

import (
	"math"
	"net/http"
	"sync"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
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
}

type WeightedServiceState struct {
	States map[string]*WeightedState
	Lock   sync.Mutex
}

func NewLoadBalancer(healthChecker *HealthChecker) *LoadBalancer {
	return &LoadBalancer{healthChecker: healthChecker}
}

// Gets the index of upstream to forward the request to based on the load balancing algorithm
func (lb *LoadBalancer) GetNextUpstream(service model.Service, clientIP string) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}
	switch service.LoadBalancingStrategy {
	case model.RoundRobin:
		return lb.handleRoundRobin(service)
	case model.Weighted:
		return lb.handleWeighted(service)
	case model.IPHash, model.ConsistentHashing:
		return lb.handleIPHash(service, clientIP)
	case model.LeastConnections:
		return lb.handleLeastConnections(service)
	case model.Latency:
		return lb.handleLatency(service)
	default:
		return lb.handleRoundRobin(service)
	}
}

// GetNextUpstreamWithRequest is the enhanced version that extracts hash values from
// the full HTTP request based on upstream configuration (hash_on, hash_on_header, etc.)
// This enables session stickiness on headers, cookies, paths, and other request attributes.
func (lb *LoadBalancer) GetNextUpstreamWithRequest(service model.Service, req *http.Request, upstreamConfig *model.UpstreamConfig) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}

	switch service.LoadBalancingStrategy {
	case model.RoundRobin:
		return lb.handleRoundRobin(service)
	case model.Weighted:
		return lb.handleWeighted(service)
	case model.IPHash, model.ConsistentHashing:
		// Extract hash value based on upstream configuration
		hashValue, _ := ExtractHashValue(req, upstreamConfig)
		return lb.handleIPHash(service, hashValue)
	case model.LeastConnections:
		return lb.handleLeastConnections(service)
	case model.Latency:
		return lb.handleLatency(service)
	default:
		return lb.handleRoundRobin(service)
	}
}

// GetNextUpstreamWithRetry selects the next upstream while excluding failed ones
// This implements Kong's failedAddresses pattern for intelligent retry routing.
//
// Kong reference: kong/runloop/balancer/latency.lua (lines 187-195)
// - Filters out addresses that have already failed for this request
// - Falls back to including all addresses if all have failed
func (lb *LoadBalancer) GetNextUpstreamWithRetry(service model.Service, clientIP string, failedUpstreams map[string]bool) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}

	// If no failed upstreams, use normal selection
	if len(failedUpstreams) == 0 {
		return lb.GetNextUpstream(service, clientIP)
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
		return lb.GetNextUpstream(service, clientIP)
	}

	// Select from non-failed upstreams based on algorithm
	switch service.LoadBalancingStrategy {
	case model.LeastConnections:
		return lb.handleLeastConnectionsExcluding(service, failedUpstreams)
	case model.Latency:
		return lb.handleLatencyExcluding(service, failedUpstreams)
	default:
		// For other algorithms, filter and select best available
		return lb.handleRoundRobinExcluding(service, failedUpstreams)
	}
}

// handleRoundRobinExcluding selects next upstream excluding failed ones
func (lb *LoadBalancer) handleRoundRobinExcluding(service model.Service, failedUpstreams map[string]bool) int {
	// Get available indices
	var availableIndices []int
	for i, u := range service.Upstreams {
		if !failedUpstreams[u.Id] {
			// Also check health
			if service.Health.Enabled {
				if state, exists := lb.healthChecker.serviceHealthMap[service.Name][u.Id]; exists {
					if state.Status != Healthy {
						continue
					}
				}
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
func (lb *LoadBalancer) handleLeastConnectionsExcluding(service model.Service, failedUpstreams map[string]bool) int {
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

// handleLatencyExcluding selects best latency upstream excluding failed ones
func (lb *LoadBalancer) handleLatencyExcluding(service model.Service, failedUpstreams map[string]bool) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}

	// Get available upstream indices (healthy and not failed)
	var candidateIndices []int
	for i, u := range service.Upstreams {
		if failedUpstreams[u.Id] {
			continue
		}
		if service.Health.Enabled {
			if state, exists := lb.healthChecker.serviceHealthMap[service.Name][u.Id]; exists {
				if state.Status != Healthy {
					continue
				}
			}
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
		return lb.handleIPHash(service, clientIP)
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

func (lb *LoadBalancer) handleLeastConnections(service model.Service) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}
	if len(service.Upstreams) == 1 {
		return 0
	}

	// Use binary heap for large upstream pools (O(log N) vs O(N))
	// Kong uses binary heap for all sizes, but for small pools linear is fine
	const heapThreshold = 10
	if len(service.Upstreams) >= heapThreshold {
		return lb.handleLeastConnectionsHeap(service)
	}

	// For small pools, use simple linear selection (O(N))
	return lb.handleLeastConnectionsLinear(service)
}

// handleLeastConnectionsHeap uses binary heap for O(log N) selection
// Used for large upstream pools (>= 10 upstreams)
func (lb *LoadBalancer) handleLeastConnectionsHeap(service model.Service) int {
	h := GetOrCreateLeastConnHeap(service)

	if service.Health.Enabled {
		// Build set of unhealthy upstreams to exclude
		unhealthy := make(map[string]bool)
		for _, u := range service.Upstreams {
			if state, exists := lb.healthChecker.serviceHealthMap[service.Name][u.Id]; exists {
				if state.Status != Healthy {
					unhealthy[u.Id] = true
				}
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

func (lb *LoadBalancer) handleIPHash(service model.Service, clientIP string) int {
	// Get or create consistent hash ring for this service
	ring, _ := consistentHashCache.LoadOrStore(service.Name, NewConsistentHashRing(service))
	consistentRing := ring.(*ConsistentHashRing)

	// If health check is not enabled, just use the consistent hash ring directly
	if !service.Health.Enabled {
		if len(service.Upstreams) == 1 {
			return 0
		}

		// Get the upstream from the ring using client IP
		upstream := consistentRing.GetUpstream(clientIP)

		// Find the index of the upstream in the service's upstreams
		for i, u := range service.Upstreams {
			if u.Id == upstream.Id {
				return i
			}
		}
		return 0
	}

	// Health check is enabled, so we need to get the healthy upstreams
	healthyUpstreams := lb.healthChecker.GetHealthyUpstreams(service)
	if len(healthyUpstreams) == 0 {
		return model.NoUpstreamsAvailable
	}

	if len(service.Upstreams) == 1 {
		return 0
	}

	// Get the upstream from the ring using client IP
	upstream := consistentRing.GetUpstream(clientIP)

	// Check if the selected upstream is healthy
	for i, u := range service.Upstreams {
		if u.Id == upstream.Id {
			if state, exists := lb.healthChecker.serviceHealthMap[service.Name][u.Id]; exists {
				if state.Status == Healthy {
					return i
				}
			}
		}
	}

	// If the selected upstream is not healthy, find the first healthy one
	for i, u := range service.Upstreams {
		if state, exists := lb.healthChecker.serviceHealthMap[service.Name][u.Id]; exists {
			if state.Status == Healthy {
				return i
			}
		}
	}

	return model.NoUpstreamsAvailable
}

func (lb *LoadBalancer) handleRoundRobin(service model.Service) int {

	// TODO: probably refactor this code, it's a bit messy
	// Health check is not enabled, so we just round robin through all upstreams
	if !service.Health.Enabled {
		if len(service.Upstreams) == 1 {
			return 0
		}

		currentVal, _ := roundRobinCache.LoadOrStore(service.Name, 0)
		currentIndex := currentVal.(int)

		nextIndex := (currentIndex + 1) % len(service.Upstreams)
		roundRobinCache.Store(service.Name, nextIndex)

		return currentIndex
	}

	// Health check is enabled, so we need to get the healthy upstreams to round robin through
	healthyUpstreams := lb.healthChecker.GetHealthyUpstreams(service)
	if len(healthyUpstreams) == 0 {
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

		// Check if upstream is healthy
		if state, exists := lb.healthChecker.serviceHealthMap[service.Name][upstream.Id]; exists {
			if state.Status == Healthy {
				// Store the next index for subsequent requests
				nextIndex := (candidateIndex + 1) % numUpstreams
				roundRobinCache.Store(service.Name, nextIndex)
				return candidateIndex
			}
		}
	}

	// No healthy upstreams found,
	return model.NoUpstreamsAvailable
}

func (lb *LoadBalancer) handleWeighted(service model.Service) int {
	if len(service.Upstreams) == 0 {
		return model.NoUpstreamsAvailable
	}
	if len(service.Upstreams) == 1 {
		return 0
	}

	// Filter healthy upstreams first if health check is enabled
	var candidateIndices []int
	if service.Health.Enabled {
		for i, u := range service.Upstreams {
			// Check if upstream is healthy
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
		// All upstreams are candidates
		for i := range service.Upstreams {
			candidateIndices = append(candidateIndices, i)
		}
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
			}
			wrapper.States[u.Id] = state
		}

		// Update effective weight if config changed
		configuredWeight := u.Weight
		if configuredWeight <= 0 {
			configuredWeight = 1
		}
		state.EffectiveWeight = configuredWeight

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
