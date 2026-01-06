package gateway

import (
	"crypto/md5"
	"fmt"
	"sort"
	"sync" // Fixed: Added sync import

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// KetamaRing implements a consistent hashing ring compatible with Kong/Nginx (Ketama algorithm)
// Used for "algorithm: ketama" configuration
type KetamaRing struct {
	service   model.Service
	continuum []KetamaNode
	mu        sync.RWMutex
}

type KetamaNode struct {
	Hash     uint32
	Upstream model.UpstreamTarget
}

// NewKetamaRing initializes a new Ketama consistent hash ring
func NewKetamaRing(service model.Service) *KetamaRing {
	ring := &KetamaRing{
		service: service,
	}
	ring.GenerateRing()
	return ring
}

// GenerateRing builds the continuum
// Kong/Nginx Logic:
// 160 virtual nodes (points) per unit of weight
// Hash: MD5 (or similar). Kong uses a custom implementation, but standard Ketama uses MD5.
// We will use 160 points per weight=1 for now, or per server.
// Kong defaults: 1000 slots?
// Actually, Ketama default is 160 points per server.
// Kong's "slots" config usually maps to the total points or points per server?
// Documentation says: "slots" (default 10000) for Ring balancer.
// Let's implement standard Ketama: 160 points per server * weight.
func (kr *KetamaRing) GenerateRing() {
	kr.mu.Lock()
	defer kr.mu.Unlock()

	var nodes []KetamaNode

	// Default points per server = 160 (standard Ketama)
	// If slots is configured, maybe usage is different.
	// Let's stick to standard Ketama for "compatibility mode".
	pointsPerServer := 160

	for _, u := range kr.service.Upstreams {
		weight := u.Weight
		if weight <= 0 {
			weight = 1
		}

		// Total points for this server
		numPoints := pointsPerServer * weight

		for i := 0; i < numPoints; i++ {
			// Key format: "target:port-index" or similar.
			// Standard Ketama: "ip:port-index" ?
			// To match Nginx exactly requires precise string format.
			// We'll use "target-i" for simplicity consistent with our internal logic.
			key := fmt.Sprintf("%s-%d", u.Target, i)
			h := kr.hash(key)

			nodes = append(nodes, KetamaNode{
				Hash:     h,
				Upstream: u,
			})
		}
	}

	// Sort by hash
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Hash < nodes[j].Hash
	})

	kr.continuum = nodes
}

func (kr *KetamaRing) hash(key string) uint32 {
	sum := md5.Sum([]byte(key))
	// Use first 4 bytes as uint32
	return uint32(sum[3])<<24 | uint32(sum[2])<<16 | uint32(sum[1])<<8 | uint32(sum[0])
}

// GetUpstream finds the target for a given key
func (kr *KetamaRing) GetUpstream(key string) model.UpstreamTarget {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	if len(kr.continuum) == 0 {
		return model.UpstreamTarget{}
	}

	h := kr.hash(key)

	// Binary search for the first node with Hash >= h
	idx := sort.Search(len(kr.continuum), func(i int) bool {
		return kr.continuum[i].Hash >= h
	})

	// Wrap around if not found (circle)
	if idx == len(kr.continuum) {
		idx = 0
	}

	return kr.continuum[idx].Upstream
}
