package gateway

import (
	"testing"
	"time"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestLoadBalancer_RoundRobin_UnhealthyUpstreams(t *testing.T) {
	// Create test service with multiple upstreams
	service := model.Service{
		Name: "test-service",
		Health: model.Health{
			Enabled: true,
			Path:    "/mock",
		},
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081"},
			{Id: "upstream2", Target: "localhost:8082"},
			{Id: "upstream3", Target: "localhost:8083"},
		},
		LoadBalancingStrategy: model.RoundRobin,
	}

	tests := []struct {
		name             string
		healthStatuses   map[string]map[string]*UpstreamHealthState // service -> upstream -> status
		expectedIndexes  []int                                      // sequence of expected indexes
		expectNoUpstream bool
	}{
		{
			name: "All upstreams healthy",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Healthy},
					"upstream2": {Status: Healthy},
					"upstream3": {Status: Healthy},
				},
			},
			expectedIndexes: []int{0, 1, 2, 0}, // Should round robin through all
		},
		{
			name: "One upstream unhealthy",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Healthy},
					"upstream2": {Status: Unhealthy},
					"upstream3": {Status: Healthy},
				},
			},
			expectedIndexes: []int{0, 2, 0, 2}, // Should skip index 1
		},
		{
			name: "Two upstreams unhealthy",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Unhealthy},
					"upstream2": {Status: Unhealthy},
					"upstream3": {Status: Healthy},
				},
			},
			expectedIndexes: []int{2, 2, 2, 2}, // Should always return index 2
		},
		{
			name: "All upstreams unhealthy",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Unhealthy},
					"upstream2": {Status: Unhealthy},
					"upstream3": {Status: Unhealthy},
				},
			},
			expectNoUpstream: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the round robin cache before each test
			ResetLoadBalancers()

			// Create health checker with mock statuses
			healthChecker := &HealthChecker{
				serviceHealthMap: tt.healthStatuses,
			}

			lb := NewLoadBalancer(healthChecker)

			if tt.expectNoUpstream {
				// Test that we get NoUpstreamsAvailable when all are unhealthy
				result := lb.GetNextUpstream(service, "", nil)
				assert.Equal(t, model.NoUpstreamsAvailable, result,
					"Expected NoUpstreamsAvailable when all upstreams are unhealthy")
			} else {
				// Test the sequence of returned indexes
				for _, expectedIdx := range tt.expectedIndexes {
					result := lb.GetNextUpstream(service, "", nil)
					assert.Equal(t, expectedIdx, result,
						"Expected upstream index %d but got %d", expectedIdx, result)
				}
			}
		})
	}
}

func TestLoadBalancer_RoundRobin_SingleUpstream(t *testing.T) {
	// Test service with single upstream
	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081"},
		},
		Health: model.Health{
			Enabled: true,
			Path:    "/mock",
		},
		LoadBalancingStrategy: model.RoundRobin,
	}

	tests := []struct {
		name           string
		healthStatus   HealthStatus
		expectedResult int
	}{
		{
			name:           "Single healthy upstream",
			healthStatus:   Healthy,
			expectedResult: 0,
		},
		{
			name:           "Single unhealthy upstream",
			healthStatus:   Unhealthy,
			expectedResult: model.NoUpstreamsAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetLoadBalancers()

			healthChecker := &HealthChecker{
				serviceHealthMap: map[string]map[string]*UpstreamHealthState{
					"test-service": {
						"upstream1": {Status: tt.healthStatus},
					},
				},
			}

			lb := NewLoadBalancer(healthChecker)
			result := lb.GetNextUpstream(service, "", nil)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestLoadBalancer_RoundRobin_HealthStateTransitions(t *testing.T) {
	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081"},
			{Id: "upstream2", Target: "localhost:8082"},
		},
		Health: model.Health{
			Enabled: true,
			Path:    "/mock",
		},
		LoadBalancingStrategy: model.RoundRobin,
	}

	healthChecker := &HealthChecker{
		serviceHealthMap: map[string]map[string]*UpstreamHealthState{
			"test-service": {
				"upstream1": {Status: Healthy},
				"upstream2": {Status: Healthy},
			},
		},
	}

	lb := NewLoadBalancer(healthChecker)
	ResetLoadBalancers()

	// Initially both healthy, should round robin
	assert.Equal(t, 0, lb.GetNextUpstream(service, "", nil))
	assert.Equal(t, 1, lb.GetNextUpstream(service, "", nil))

	// Mark upstream1 as unhealthy
	healthChecker.serviceHealthMap["test-service"]["upstream1"] = &UpstreamHealthState{Status: Unhealthy}

	// Should only return upstream2
	assert.Equal(t, 1, lb.GetNextUpstream(service, "", nil))
	assert.Equal(t, 1, lb.GetNextUpstream(service, "", nil))

	// Mark upstream1 as healthy again
	healthChecker.serviceHealthMap["test-service"]["upstream1"] = &UpstreamHealthState{Status: Healthy}

	// Should resume round robin from last position
	assert.Equal(t, 0, lb.GetNextUpstream(service, "", nil))
	assert.Equal(t, 1, lb.GetNextUpstream(service, "", nil))

	// Mark both as unhealthy
	healthChecker.serviceHealthMap["test-service"]["upstream1"] = &UpstreamHealthState{Status: Unhealthy}
	healthChecker.serviceHealthMap["test-service"]["upstream2"] = &UpstreamHealthState{Status: Unhealthy}

	// Should return no upstreams available
	assert.Equal(t, model.NoUpstreamsAvailable, lb.GetNextUpstream(service, "", nil))
}

func TestLoadBalancer_RoundRobin_HealthCheckDisabled(t *testing.T) {
	// Create test service with health check disabled
	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081"},
			{Id: "upstream2", Target: "localhost:8082"},
			{Id: "upstream3", Target: "localhost:8083"},
		},
		Health: model.Health{
			Enabled: false, // Health check disabled
			Path:    "/health",
		},
		LoadBalancingStrategy: model.RoundRobin,
	}

	tests := []struct {
		name            string
		healthStatuses  map[string]map[string]*UpstreamHealthState
		expectedIndexes []int
	}{
		{
			name: "Should round-robin if health check is not turned on",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: NotAvailable},
					"upstream2": {Status: NotAvailable},
					"upstream3": {Status: NotAvailable},
				},
			},
			expectedIndexes: []int{0, 1, 2, 0}, // Should round robin through all despite health status
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetLoadBalancers()

			// Create health checker with mock statuses
			healthChecker := &HealthChecker{
				serviceHealthMap: tt.healthStatuses,
			}

			lb := NewLoadBalancer(healthChecker)

			// Test the sequence of returned indexes
			for _, expectedIdx := range tt.expectedIndexes {
				result := lb.GetNextUpstream(service, "", nil)
				assert.Equal(t, expectedIdx, result,
					"Expected upstream index %d but got %d when health check disabled", expectedIdx, result)
			}
		})
	}
}

func TestLoadBalancer_IPHash_ConsistentHashing(t *testing.T) {
	// Create test service with multiple upstreams
	service := model.Service{
		Name: "test-service",
		Health: model.Health{
			Enabled: false,
			Path:    "/mock",
		},
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081"},
			{Id: "upstream2", Target: "localhost:8082"},
			{Id: "upstream3", Target: "localhost:8083"},
		},
		LoadBalancingStrategy: model.IPHash,
	}

	tests := []struct {
		name     string
		clientIP string
		runCount int // Number of times to run the test to verify consistency
	}{
		{
			name:     "Same IP should always map to same upstream",
			clientIP: "192.168.1.1",
			runCount: 10,
		},
		{
			name:     "Different IP should potentially map to different upstream",
			clientIP: "192.168.1.2",
			runCount: 10,
		},
		{
			name:     "IPv6 address handling",
			clientIP: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			runCount: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetLoadBalancers()

			healthChecker := &HealthChecker{
				serviceHealthMap: map[string]map[string]*UpstreamHealthState{},
			}

			lb := NewLoadBalancer(healthChecker)

			// First run to get initial mapping
			firstIndex := lb.handleIPHash(service, tt.clientIP, nil)

			// Verify the index is valid
			assert.GreaterOrEqual(t, firstIndex, 0)
			assert.Less(t, firstIndex, len(service.Upstreams))

			// Run multiple times to verify consistency
			for i := 0; i < tt.runCount; i++ {
				index := lb.handleIPHash(service, tt.clientIP, nil)
				// Same IP should always map to the same upstream
				assert.Equal(t, firstIndex, index,
					"Same IP should map to same upstream on multiple calls")
			}

			// If we have a different test case, verify it maps to a potentially different upstream
			if len(tests) > 1 {
				differentIP := "192.168.1.100"
				if tt.clientIP == differentIP {
					differentIP = "192.168.1.200"
				}
				differentIndex := lb.handleIPHash(service, differentIP, nil)
				// Note: There's a small chance this could fail if the hash happens to map to the same upstream
				// This is expected and acceptable in a real-world scenario
				if differentIndex == firstIndex {
					t.Logf("Different IP mapped to same upstream (this is possible but rare)")
				}
			}
		})
	}
}

func TestLoadBalancer_IPHash_HealthCheck(t *testing.T) {
	// Create test service with multiple upstreams
	service := model.Service{
		Name: "test-service",
		Health: model.Health{
			Enabled: true,
			Path:    "/mock",
		},
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081"},
			{Id: "upstream2", Target: "localhost:8082"},
			{Id: "upstream3", Target: "localhost:8083"},
		},
		LoadBalancingStrategy: model.IPHash,
	}

	tests := []struct {
		name             string
		clientIP         string
		healthStatuses   map[string]map[string]*UpstreamHealthState
		expectNoUpstream bool
	}{
		{
			name:     "All upstreams healthy",
			clientIP: "192.168.1.1",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Healthy},
					"upstream2": {Status: Healthy},
					"upstream3": {Status: Healthy},
				},
			},
			expectNoUpstream: false,
		},
		{
			name:     "Some upstreams unhealthy",
			clientIP: "192.168.1.2",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Unhealthy},
					"upstream2": {Status: Healthy},
					"upstream3": {Status: Healthy},
				},
			},
			expectNoUpstream: false,
		},
		{
			name:     "All upstreams unhealthy",
			clientIP: "192.168.1.3",
			healthStatuses: map[string]map[string]*UpstreamHealthState{
				"test-service": {
					"upstream1": {Status: Unhealthy},
					"upstream2": {Status: Unhealthy},
					"upstream3": {Status: Unhealthy},
				},
			},
			expectNoUpstream: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetLoadBalancers()

			healthChecker := &HealthChecker{
				serviceHealthMap: tt.healthStatuses,
			}

			lb := NewLoadBalancer(healthChecker)
			result := lb.handleIPHash(service, tt.clientIP, nil)

			if tt.expectNoUpstream {
				assert.Equal(t, model.NoUpstreamsAvailable, result,
					"Expected no available upstreams")
			} else {
				assert.GreaterOrEqual(t, result, 0)
				assert.Less(t, result, len(service.Upstreams))

				// Verify the selected upstream is healthy
				upstream := service.Upstreams[result]
				state := tt.healthStatuses["test-service"][upstream.Id]
				assert.Equal(t, Healthy, state.Status,
					"Selected upstream should be healthy")
			}
		})
	}
}

func TestLoadBalancer_IPHash_SingleUpstream(t *testing.T) {
	// Test service with single upstream
	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081"},
		},
		Health: model.Health{
			Enabled: true,
			Path:    "/mock",
		},
		LoadBalancingStrategy: model.IPHash,
	}

	tests := []struct {
		name           string
		clientIP       string
		healthStatus   HealthStatus
		expectedResult int
	}{
		{
			name:           "Single healthy upstream",
			clientIP:       "192.168.1.1",
			healthStatus:   Healthy,
			expectedResult: 0,
		},
		{
			name:           "Single unhealthy upstream",
			clientIP:       "192.168.1.2",
			healthStatus:   Unhealthy,
			expectedResult: model.NoUpstreamsAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetLoadBalancers()

			healthChecker := &HealthChecker{
				serviceHealthMap: map[string]map[string]*UpstreamHealthState{
					"test-service": {
						"upstream1": {Status: tt.healthStatus},
					},
				},
			}

			lb := NewLoadBalancer(healthChecker)
			result := lb.handleIPHash(service, tt.clientIP, nil)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestLoadBalancer_GetCurrentUpstream_IPHash(t *testing.T) {
	// Create test service with multiple upstreams
	service := model.Service{
		Name: "test-service",
		Health: model.Health{
			Enabled: false,
			Path:    "/mock",
		},
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081"},
			{Id: "upstream2", Target: "localhost:8082"},
			{Id: "upstream3", Target: "localhost:8083"},
		},
		LoadBalancingStrategy: model.IPHash,
	}

	tests := []struct {
		name     string
		clientIP string
	}{
		{
			name:     "Should return same upstream for same IP",
			clientIP: "192.168.1.1",
		},
		{
			name:     "Should handle IPv6",
			clientIP: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetLoadBalancers()

			healthChecker := &HealthChecker{
				serviceHealthMap: map[string]map[string]*UpstreamHealthState{},
			}

			lb := NewLoadBalancer(healthChecker)

			// Get initial upstream
			initialUpstream := lb.GetCurrentUpstream(service, tt.clientIP)

			// Verify the index is valid
			assert.GreaterOrEqual(t, initialUpstream, 0)
			assert.Less(t, initialUpstream, len(service.Upstreams))

			// Multiple calls should return the same upstream for the same IP
			for i := 0; i < 5; i++ {
				currentUpstream := lb.GetCurrentUpstream(service, tt.clientIP)
				assert.Equal(t, initialUpstream, currentUpstream,
					"Same IP should map to same upstream on multiple GetCurrentUpstream calls")
			}

			// Different IP should potentially map to different upstream
			differentIP := "192.168.1.100"
			if tt.clientIP == differentIP {
				differentIP = "192.168.1.200"
			}
			differentUpstream := lb.GetCurrentUpstream(service, differentIP)

			// Note: There's a small chance this could be equal due to hash collision
			if differentUpstream == initialUpstream {
				t.Logf("Different IP mapped to same upstream (this is possible but rare)")
			}
		})
	}
}

func TestLoadBalancer_Weighted(t *testing.T) {
	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081", Weight: 2},
			{Id: "upstream2", Target: "localhost:8082", Weight: 1},
		},
		Health: model.Health{
			Enabled: false,
		},
		LoadBalancingStrategy: model.Weighted,
	}

	tests := []struct {
		name            string
		expectedIndexes []int
	}{
		{
			name: "Weighted 2:1 distribution",
			// With SWRR:
			// 1. A(2), B(1). A+=2, B+=1. Currents: A=2, B=1. Max: A=2. Pick A. A-=3. Currents: A=-1, B=1.
			// 2. A+=2, B+=1. Currents: A=1, B=2. Max: B=2. Pick B. B-=3. Currents: A=1, B=-1.
			// 3. A+=2, B+=1. Currents: A=3, B=0. Max: A=3. Pick A. A-=3. Currents: A=0, B=0.
			// Sequence: 0, 1, 0
			expectedIndexes: []int{0, 1, 0, 0, 1, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetLoadBalancers()
			healthChecker := &HealthChecker{
				serviceHealthMap: map[string]map[string]*UpstreamHealthState{},
			}
			lb := NewLoadBalancer(healthChecker)

			// Pre-warm the cache to bypass Slow Start
			wrapper := &WeightedServiceState{
				States: make(map[string]*WeightedState),
			}
			past := time.Now().Add(-2 * time.Hour) // Long enough to be full weight
			wrapper.States["upstream1"] = &WeightedState{
				FirstSeenAt: past,
			}
			wrapper.States["upstream2"] = &WeightedState{
				FirstSeenAt: past,
			}
			weightedCache.Store(service.Name, wrapper)

			for i, expectedIdx := range tt.expectedIndexes {
				result := lb.GetNextUpstream(service, "", nil)
				assert.Equal(t, expectedIdx, result,
					"Step %d: Expected upstream index %d but got %d", i, expectedIdx, result)
			}
		})
	}
}

func TestLoadBalancer_Weighted_HealthCheck(t *testing.T) {
	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081", Weight: 10},
			{Id: "upstream2", Target: "localhost:8082", Weight: 1},
		},
		Health: model.Health{
			Enabled: true,
			Path:    "/health",
		},
		LoadBalancingStrategy: model.Weighted,
	}

	// Health checker with upstream1 UNHEALTHY
	healthChecker := &HealthChecker{
		serviceHealthMap: map[string]map[string]*UpstreamHealthState{
			"test-service": {
				"upstream1": {Status: Unhealthy},
				"upstream2": {Status: Healthy},
			},
		},
	}

	ResetLoadBalancers()
	lb := NewLoadBalancer(healthChecker)

	// Since upstream1 is unhealthy, ALL requests must go to upstream2 (index 1)
	for i := 0; i < 5; i++ {
		result := lb.GetNextUpstream(service, "", nil)
		assert.Equal(t, 1, result, "Should always pick healthy upstream")
	}
}

func TestLoadBalancer_LeastConnections_WeightAware(t *testing.T) {
	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081", Weight: 10}, // High weight
			{Id: "upstream2", Target: "localhost:8082", Weight: 1},  // Low weight
		},
		Health: model.Health{
			Enabled: false,
		},
		LoadBalancingStrategy: model.LeastConnections,
	}

	tests := []struct {
		name             string
		preSetup         func()
		expectedUpstream int
		description      string
	}{
		{
			name: "Same connections, higher weight wins",
			preSetup: func() {
				// Both have 0 connections, upstream1 has higher weight
				// Score for upstream1: (0+1)/10 = 0.1
				// Score for upstream2: (0+1)/1 = 1.0
				// upstream1 wins (lower score)
			},
			expectedUpstream: 0,
			description:      "With equal connections, higher weight should win",
		},
		{
			name: "Lower connections wins despite weight",
			preSetup: func() {
				// Give upstream1 many connections
				for i := 0; i < 20; i++ {
					IncrementActiveConnections("test-service", "upstream1")
				}
				// upstream1: (20+1)/10 = 2.1
				// upstream2: (0+1)/1 = 1.0
				// upstream2 wins
			},
			expectedUpstream: 1,
			description:      "Lower connection count can overcome higher weight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetLoadBalancers()

			healthChecker := &HealthChecker{
				serviceHealthMap: map[string]map[string]*UpstreamHealthState{},
			}
			lb := NewLoadBalancer(healthChecker)

			if tt.preSetup != nil {
				tt.preSetup()
			}

			result := lb.GetNextUpstream(service, "", nil)
			assert.Equal(t, tt.expectedUpstream, result, tt.description)
		})
	}
}

func TestLoadBalancer_Latency_EWMA(t *testing.T) {
	service := model.Service{
		Name: "test-latency-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "fast-upstream", Target: "localhost:8081", Weight: 1},
			{Id: "slow-upstream", Target: "localhost:8082", Weight: 1},
		},
		Health: model.Health{
			Enabled: false,
		},
		LoadBalancingStrategy: model.Latency,
	}

	tests := []struct {
		name             string
		preSetup         func()
		expectedUpstream int
		description      string
	}{
		{
			name: "No latency data - should return first",
			preSetup: func() {
				// No setup - fresh state
			},
			expectedUpstream: 0,
			description:      "With no latency data, should return first upstream",
		},
		{
			name: "Fast upstream should be preferred",
			preSetup: func() {
				// Record fast latency for first upstream
				RecordLatency("test-latency-service", "fast-upstream", 10*time.Millisecond)
				RecordLatency("test-latency-service", "fast-upstream", 15*time.Millisecond)

				// Record slow latency for second upstream
				RecordLatency("test-latency-service", "slow-upstream", 500*time.Millisecond)
				RecordLatency("test-latency-service", "slow-upstream", 600*time.Millisecond)
			},
			expectedUpstream: 0,
			description:      "Upstream with lower latency should be preferred",
		},
		{
			name: "Slow upstream becomes fast - should switch",
			preSetup: func() {
				// Initially slow upstream was slow
				RecordLatency("test-latency-service", "slow-upstream", 500*time.Millisecond)

				// Now fast upstream is slow
				RecordLatency("test-latency-service", "fast-upstream", 1000*time.Millisecond)

				// And slow upstream is now fast
				RecordLatency("test-latency-service", "slow-upstream", 5*time.Millisecond)
			},
			expectedUpstream: 1,
			description:      "EWMA should adapt to changing latency patterns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetLoadBalancers()

			healthChecker := &HealthChecker{
				serviceHealthMap: map[string]map[string]*UpstreamHealthState{},
			}
			lb := NewLoadBalancer(healthChecker)

			if tt.preSetup != nil {
				tt.preSetup()
			}

			result := lb.GetNextUpstream(service, "", nil)
			assert.Equal(t, tt.expectedUpstream, result, tt.description)
		})
	}
}

func TestLoadBalancer_Latency_HealthCheck(t *testing.T) {
	service := model.Service{
		Name: "test-latency-health-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081", Weight: 1},
			{Id: "upstream2", Target: "localhost:8082", Weight: 1},
			{Id: "upstream3", Target: "localhost:8083", Weight: 1},
		},
		Health: model.Health{
			Enabled: true,
			Path:    "/health",
		},
		LoadBalancingStrategy: model.Latency,
	}

	ResetLoadBalancers()

	// Record multiple latency samples to establish clear ordering
	// Record more samples to overcome slow-start EWMA initialization
	for i := 0; i < 5; i++ {
		RecordLatency("test-latency-health-service", "upstream1", 10*time.Millisecond)
		RecordLatency("test-latency-health-service", "upstream2", 50*time.Millisecond)
		RecordLatency("test-latency-health-service", "upstream3", 200*time.Millisecond)
	}

	// Mark upstream1 as unhealthy
	healthChecker := &HealthChecker{
		serviceHealthMap: map[string]map[string]*UpstreamHealthState{
			"test-latency-health-service": {
				"upstream1": {Status: Unhealthy},
				"upstream2": {Status: Healthy},
				"upstream3": {Status: Healthy},
			},
		},
	}

	lb := NewLoadBalancer(healthChecker)
	result := lb.GetNextUpstream(service, "", nil)

	// Should pick upstream2 (index 1) - fastest among healthy ones
	assert.Equal(t, 1, result, "Should pick fastest healthy upstream (upstream2)")
}

func TestLoadBalancer_ConsistentHashing_Algorithm(t *testing.T) {
	service := model.Service{
		Name: "test-consistent-hashing",
		Upstreams: []model.UpstreamTarget{
			{Id: "upstream1", Target: "localhost:8081"},
			{Id: "upstream2", Target: "localhost:8082"},
			{Id: "upstream3", Target: "localhost:8083"},
		},
		Health: model.Health{
			Enabled: false,
		},
		LoadBalancingStrategy: model.ConsistentHashing,
	}

	ResetLoadBalancers()

	healthChecker := &HealthChecker{
		serviceHealthMap: map[string]map[string]*UpstreamHealthState{},
	}
	lb := NewLoadBalancer(healthChecker)

	// Same key should always map to same upstream
	firstResult := lb.GetNextUpstream(service, "session-123", nil)
	for i := 0; i < 10; i++ {
		result := lb.GetNextUpstream(service, "session-123", nil)
		assert.Equal(t, firstResult, result, "Same key should map to same upstream")
	}

	// Different keys may map to different upstreams
	differentResult := lb.GetNextUpstream(service, "session-456", nil)
	// Note: Could be same due to hash collision, that's expected
	t.Logf("session-123 maps to %d, session-456 maps to %d", firstResult, differentResult)
}
