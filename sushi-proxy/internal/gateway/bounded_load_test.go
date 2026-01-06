package gateway

import (
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

func TestLoadBalancer_BoundedLoad(t *testing.T) {
	// Setup
	hc := NewHealthChecker()
	lb := NewLoadBalancer(hc)

	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "u1", Target: "127.0.0.1:8001", Weight: 1},
			{Id: "u2", Target: "127.0.0.1:8002", Weight: 1},
		},
		LoadBalancingStrategy: model.ConsistentHashing,
	}

	// Reset caches
	ResetLoadBalancers()

	// Simulate high load on u1 (index 0)
	// We need 'total > 10' for bounded load to kick in.
	// Let's set u1 to 20 connections, u2 to 0.
	// Avg = 10. Max = 12.5. u1 > Max.
	// Requests hashing to u1 should spill to u2.

	for i := 0; i < 20; i++ {
		IncrementActiveConnections(service.Name, "u1")
	}

	// Verify u1 is overloaded
	u1Load := GetActiveConnections(service.Name, "u1")
	if u1Load != 20 {
		t.Fatalf("Expected u1 load 20, got %d", u1Load)
	}

	// We need a key that hash maps to u1.
	// Since permutations are random, we iterate until we find a key that *would* go to u1 normally.
	// But GetNextUpstream calls handleIPHash which does the Bounded Check.
	// We can't easily check "what it would have been" without checking the Ring internal.
	// Instead, we just check that GetNextUpstream returns u2 (index 1) for *some* keys that should map to u1.
	// Or simpler: If we throw 100 requests at it, ALL should go to u2 because u1 is overloaded?
	// Maglev with 2 nodes:
	// If u1 is rejected, Maglev probes. If u2 is accepted, it returns u2.
	// So effectively, u1 is disabled. All traffic should go to u2.

	hits := make(map[int]int)
	for i := 0; i < 100; i++ {
		// Use varying IPs to cover the hash space
		// "192.168.1.X"
		ip := "192.168.1." + string(rune(i))
		idx := lb.GetNextUpstream(service, ip, nil)
		hits[idx]++
	}

	// Try to assert that u1 received 0 hits (or very few if my "key" logic in finding u1 was flawed)
	// Actually, u1 is overloaded (20 > 12.5). Any request mapping to u1 MUST skip u1.
	// u2 is not overloaded (0 < 12.5).
	// So u1 should get 0 new hits.

	if hits[0] > 0 {
		t.Errorf("Expected 0 hits for overloaded u1, got %d", hits[0])
	}
	if hits[1] == 0 {
		t.Errorf("Expected hits for u2, got 0")
	}
}
