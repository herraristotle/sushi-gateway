package gateway

import (
	"fmt"
	"testing"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

func TestKetamaRing_Distribution(t *testing.T) {
	service := model.Service{
		Name: "ketama-service",
		Upstreams: []model.UpstreamTarget{
			{Id: "A", Target: "10.0.0.1:80", Weight: 1},
			{Id: "B", Target: "10.0.0.2:80", Weight: 1},
			{Id: "C", Target: "10.0.0.3:80", Weight: 2}, // Double weight
		},
	}

	ring := NewKetamaRing(service)

	// Verify continuum size
	// A=160, B=160, C=320. Total = 640.
	if len(ring.continuum) != 640 {
		t.Errorf("Expected continuum size 640, got %d", len(ring.continuum))
	}

	// Verify consistent mapping
	// Same key should always map to same upstream
	key := "user-123"
	u1 := ring.GetUpstream(key)
	u2 := ring.GetUpstream(key)

	if u1.Id != u2.Id {
		t.Errorf("Ketama consistency failed: %s vs %s", u1.Id, u2.Id)
	}

	// Verify approximate distribution (10000 keys)
	// A~25%, B~25%, C~50%
	counts := make(map[string]int)
	for i := 0; i < 10000; i++ {
		k := fmt.Sprintf("key-%d", i)
		u := ring.GetUpstream(k)
		counts[u.Id]++
	}

	t.Logf("Distribution: A=%d, B=%d, C=%d", counts["A"], counts["B"], counts["C"])

	if counts["C"] < counts["A"] || counts["C"] < counts["B"] {
		t.Errorf("Expected C to have more hits than A or B due to weight")
	}
}
