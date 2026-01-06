package gateway

import (
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

func TestLoadBalancer_SubsetRouting(t *testing.T) {
	// Setup
	hc := NewHealthChecker()
	lb := NewLoadBalancer(hc)

	service := model.Service{
		Name: "test-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "A", Target: "127.0.0.1:8001", Weight: 1, Tags: []string{"v1", "prod"}},
			{Id: "B", Target: "127.0.0.1:8002", Weight: 1, Tags: []string{"v1", "prod"}},
			{Id: "C", Target: "127.0.0.1:8003", Weight: 1, Tags: []string{"v2", "canary"}},
		},
		LoadBalancingStrategy: model.RoundRobin,
	}

	t.Run("Route with no tags (default)", func(t *testing.T) {
		tags := []string{}
		// Should hit A, B, or C (no filter, RR)
		hits := make(map[string]int)
		for i := 0; i < 30; i++ {
			idx := lb.GetNextUpstream(service, "ip", tags)
			id := service.Upstreams[idx].Id
			hits[id]++
		}
		if hits["A"] == 0 || hits["B"] == 0 || hits["C"] == 0 {
			t.Errorf("Expected hits on all A, B, C. Got: %v", hits)
		}
	})

	t.Run("Route with 'v1' tag", func(t *testing.T) {
		tags := []string{"v1"}
		// Should hit A or B only. C is v2.
		hits := make(map[string]int)
		for i := 0; i < 20; i++ {
			idx := lb.GetNextUpstream(service, "ip", tags)
			id := service.Upstreams[idx].Id
			hits[id]++
		}
		if hits["C"] > 0 {
			t.Errorf("Expected 0 hits on C (v2), got %d", hits["C"])
		}
		if hits["A"] == 0 || hits["B"] == 0 {
			t.Errorf("Expected hits on both A and B. Got: %v", hits)
		}
	})

	t.Run("Route with 'canary' tag", func(t *testing.T) {
		tags := []string{"canary"}
		// Should hit C only.
		hits := make(map[string]int)
		for i := 0; i < 10; i++ {
			idx := lb.GetNextUpstream(service, "ip", tags)
			id := service.Upstreams[idx].Id
			hits[id]++
		}
		if hits["A"] > 0 || hits["B"] > 0 {
			t.Errorf("Expected 0 hits on A/B, got hits: %v", hits)
		}
		if hits["C"] != 10 {
			t.Errorf("Expected all 10 hits on C, got %d", hits["C"])
		}
	})

	t.Run("Route with 'prod' AND 'v1' tag", func(t *testing.T) {
		tags := []string{"prod", "v1"}
		// Should hit A or B.
		idx := lb.GetNextUpstream(service, "ip", tags)
		id := service.Upstreams[idx].Id
		if id != "A" && id != "B" {
			t.Errorf("Expected A or B, got %s", id)
		}
	})

	t.Run("Route with Non-existent tag", func(t *testing.T) {
		tags := []string{"v999"}
		idx := lb.GetNextUpstream(service, "ip", tags)
		if idx != model.NoUpstreamsAvailable {
			t.Errorf("Expected NoUpstreamsAvailable, got %d", idx)
		}
	})
}
