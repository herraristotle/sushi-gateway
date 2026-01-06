package gateway

import (
	"sync"

	"github.com/cespare/xxhash"
	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// Maglev Hashing Constants
const (
	// M must be a prime number. 65537 is a common choice for Maglev.
	MaglevM = 65537
)

type ConsistentHashRing struct {
	service      model.Service
	lookupTable  []int   // Lookup table of size M, stores index into service.Upstreams
	permutations [][]int // Permutations for each upstream
	mu           sync.RWMutex
}

// NewConsistentHashRing initializes a new Maglev-based consistent hash ring
func NewConsistentHashRing(service model.Service) *ConsistentHashRing {
	ring := &ConsistentHashRing{
		service: service,
	}
	ring.GenerateRing()
	return ring
}

// GenerateRing rebuilds the Maglev lookup table
func (chr *ConsistentHashRing) GenerateRing() {
	chr.mu.Lock()
	defer chr.mu.Unlock()

	numUpstreams := len(chr.service.Upstreams)
	if numUpstreams == 0 {
		chr.lookupTable = make([]int, 0)
		return
	}

	// 1. Generate Permutations
	// Permutations[i] is a slice of size M for upstream i
	chr.permutations = make([][]int, numUpstreams)
	for i, upstream := range chr.service.Upstreams {
		chr.permutations[i] = generatePermutation(upstream.Id, MaglevM)
	}

	// 2. Populate Lookup Table (The Maglev)
	chr.lookupTable = make([]int, MaglevM)
	for i := range chr.lookupTable {
		chr.lookupTable[i] = -1
	}

	// next[i] tracks the next index in the permutation key for upstream i
	next := make([]int, numUpstreams)
	n := 0 // Number of populated entries

	for {
		for i := 0; i < numUpstreams; i++ {
			c := chr.permutations[i][next[i]]
			for chr.lookupTable[c] >= 0 {
				next[i]++
				c = chr.permutations[i][next[i]]
			}
			chr.lookupTable[c] = i
			next[i]++
			n++
			if n == MaglevM {
				return
			}
		}
	}
}

// generatePermutation generates a random permutation for the backend using
// two hash values offset and skip.
// Permutation = (offset + j * skip) % M
func generatePermutation(key string, m int) []int {
	offset, skip := generateOffsetAndSkip(key, m)
	perm := make([]int, m)
	for j := 0; j < m; j++ {
		perm[j] = (offset + j*skip) % m
	}
	return perm
}

// generateOffsetAndSkip generates random offset and skip values
func generateOffsetAndSkip(key string, m int) (int, int) {
	// Use xxHash for speed
	h64 := xxhash.Sum64([]byte(key))

	// Split 64-bit hash into two 32-bit components?
	// Or just hash twice with different seeds?
	// Let's use simple bit manipulation for now.
	// offset = h % M
	// skip = (h >> 32) % (M-1) + 1

	offset := int(h64 % uint64(m))
	skip := int((h64>>32)%uint64(m-1)) + 1
	return offset, skip
}

// GetUpstream gets the upstream for a given client key (e.g. IP)
func (chr *ConsistentHashRing) GetUpstream(key string) model.UpstreamTarget {
	return chr.GetUpstreamWithFilter(key, nil)
}

// GetUpstreamWithFilter gets an upstream, skipping those rejected by the filter
func (chr *ConsistentHashRing) GetUpstreamWithFilter(key string, filter func(model.UpstreamTarget) bool) model.UpstreamTarget {
	chr.mu.RLock()
	defer chr.mu.RUnlock()

	if len(chr.lookupTable) == 0 || len(chr.service.Upstreams) == 0 {
		return model.UpstreamTarget{}
	}

	// Hash the key to find index in lookup table
	h := xxhash.Sum64([]byte(key))
	startIdx := int(h % uint64(MaglevM))

	// Probe linearly starting from hashed position until we find an accepted upstream
	// To prevent infinite loops, we stop after checking 'len(lookupTable)' times
	for i := 0; i < MaglevM; i++ {
		currIdx := (startIdx + i) % MaglevM
		upstreamIdx := chr.lookupTable[currIdx]

		if upstreamIdx < 0 || upstreamIdx >= len(chr.service.Upstreams) {
			continue // skip invalid entries
		}

		target := chr.service.Upstreams[upstreamIdx]

		// If no filter or filter allows it, return
		if filter == nil || filter(target) {
			return target
		}
	}

	// Fallback: return the primary mapping if no one is accepted (fail open or maintain consistency?)
	// Implementation choice: fail open to primary to avoid dropping request,
	// unless we want to return empty to signal 503.
	// Let's return the primary mapping as failsafe.
	upstreamIdx := chr.lookupTable[startIdx]
	if upstreamIdx >= 0 && upstreamIdx < len(chr.service.Upstreams) {
		return chr.service.Upstreams[upstreamIdx]
	}

	return model.UpstreamTarget{}
}

func (chr *ConsistentHashRing) AddNewUpstream(upstream model.UpstreamTarget) {
	// For Maglev, we generally rebuild the whole table on change.
	// Append and rebuild.
	chr.mu.Lock()
	// Temporarily unlock to avoid deadlock since GenerateRing locks,
	// but we actually need to modify service.Upstreams under lock?
	// The struct has a copy of service?
	// The service struct in ConsistentHashRing might be a copy.
	// Let's modify our local slice.

	// NOTE: This implementation assumes chr.service used in GenerateRing is the one we modify here.
	chr.service.Upstreams = append(chr.service.Upstreams, upstream)
	chr.mu.Unlock() // GenerateRing will re-lock

	chr.GenerateRing()
}

func (chr *ConsistentHashRing) RemoveUpstream(upstreamId string) {
	chr.mu.Lock()

	// Find and remove
	found := -1
	for i, u := range chr.service.Upstreams {
		if u.Id == upstreamId {
			found = i
			break
		}
	}

	if found != -1 {
		// Remove from slice
		chr.service.Upstreams = append(chr.service.Upstreams[:found], chr.service.Upstreams[found+1:]...)
	}

	chr.mu.Unlock()
	chr.GenerateRing()
}
