package gateway

import (
	"container/heap"
	"sync"

	"github.com/rawsashimi1604/sushi-gateway/sushi-proxy/internal/model"
)

// LeastConnectionsHeap implements a binary min-heap for O(log N) least-connections
// selection, following Kong's implementation pattern.
//
// Kong reference: kong/runloop/balancer/least_connections.lua
//   - Uses binaryheap library with score = (connections + 1) / weight
//   - Supports failed address tracking for retries
//   - Automatically updates heap on connection count changes

// UpstreamHeapItem represents an upstream in the binary heap
type UpstreamHeapItem struct {
	Index           int     // Index in service.Upstreams
	UpstreamId      string  // Upstream ID
	ConnectionCount int64   // Current active connections
	Weight          float64 // Upstream weight
	Score           float64 // Calculated score: (connections + 1) / weight
	HeapIndex       int     // Position in heap (managed by heap.Interface)
}

// LeastConnectionsHeap is a min-heap of upstreams sorted by score
type LeastConnectionsHeap struct {
	items   []*UpstreamHeapItem
	itemMap map[string]*UpstreamHeapItem // upstreamId -> item for O(1) lookup
	mu      sync.Mutex
}

// NewLeastConnectionsHeap creates a new heap for a service's upstreams
func NewLeastConnectionsHeap(upstreams []model.UpstreamTarget, serviceName string) *LeastConnectionsHeap {
	h := &LeastConnectionsHeap{
		items:   make([]*UpstreamHeapItem, 0, len(upstreams)),
		itemMap: make(map[string]*UpstreamHeapItem),
	}

	for i, u := range upstreams {
		weight := float64(u.Weight)
		if weight <= 0 {
			weight = 1
		}
		conns := GetActiveConnections(serviceName, u.Id)
		score := float64(conns+1) / weight

		item := &UpstreamHeapItem{
			Index:           i,
			UpstreamId:      u.Id,
			ConnectionCount: conns,
			Weight:          weight,
			Score:           score,
		}
		h.items = append(h.items, item)
		h.itemMap[u.Id] = item
	}

	heap.Init(h)
	return h
}

// Len implements heap.Interface
func (h *LeastConnectionsHeap) Len() int { return len(h.items) }

// Less implements heap.Interface - min-heap by score
func (h *LeastConnectionsHeap) Less(i, j int) bool {
	return h.items[i].Score < h.items[j].Score
}

// Swap implements heap.Interface
func (h *LeastConnectionsHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].HeapIndex = i
	h.items[j].HeapIndex = j
}

// Push implements heap.Interface
func (h *LeastConnectionsHeap) Push(x interface{}) {
	item := x.(*UpstreamHeapItem)
	item.HeapIndex = len(h.items)
	h.items = append(h.items, item)
	h.itemMap[item.UpstreamId] = item
}

// Pop implements heap.Interface
func (h *LeastConnectionsHeap) Pop() interface{} {
	old := h.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // avoid memory leak
	h.items = old[0 : n-1]
	item.HeapIndex = -1
	delete(h.itemMap, item.UpstreamId)
	return item
}

// Peek returns the upstream with lowest score without removing it - O(1)
func (h *LeastConnectionsHeap) Peek() *UpstreamHeapItem {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.items) == 0 {
		return nil
	}
	return h.items[0]
}

// PeekExcluding returns the upstream with lowest score excluding specified IDs - O(N) worst case
// This is used for retry tracking to avoid failed upstreams
func (h *LeastConnectionsHeap) PeekExcluding(excludeIds map[string]bool) *UpstreamHeapItem {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Pop excluded items, find first available, then re-insert
	var popped []*UpstreamHeapItem
	var result *UpstreamHeapItem

	for len(h.items) > 0 {
		top := heap.Pop(h).(*UpstreamHeapItem)
		if excludeIds != nil && excludeIds[top.UpstreamId] {
			popped = append(popped, top)
			continue
		}
		result = top
		popped = append(popped, top) // We'll re-insert it too
		break
	}

	// Re-insert all popped items
	for _, item := range popped {
		heap.Push(h, item)
	}

	return result
}

// UpdateConnection updates the connection count and re-heapifies - O(log N)
func (h *LeastConnectionsHeap) UpdateConnection(upstreamId string, delta int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	item, exists := h.itemMap[upstreamId]
	if !exists {
		return
	}

	item.ConnectionCount += delta
	if item.ConnectionCount < 0 {
		item.ConnectionCount = 0
	}
	item.Score = float64(item.ConnectionCount+1) / item.Weight

	heap.Fix(h, item.HeapIndex)
}

// GetBestUpstream returns the index of the upstream with lowest score - O(1)
func (h *LeastConnectionsHeap) GetBestUpstream() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.items) == 0 {
		return model.NoUpstreamsAvailable
	}
	return h.items[0].Index
}

// GetBestUpstreamExcluding returns best upstream excluding failed ones
// Used for retry tracking (Kong-style failedAddresses)
func (h *LeastConnectionsHeap) GetBestUpstreamExcluding(failedUpstreams map[string]bool) int {
	item := h.PeekExcluding(failedUpstreams)
	if item == nil {
		return model.NoUpstreamsAvailable
	}
	return item.Index
}

// --- Per-service heap cache ---

var leastConnHeapCache sync.Map // serviceName -> *LeastConnectionsHeap

// GetOrCreateLeastConnHeap gets or creates a heap for a service
func GetOrCreateLeastConnHeap(service model.Service) *LeastConnectionsHeap {
	val, loaded := leastConnHeapCache.LoadOrStore(service.Name,
		NewLeastConnectionsHeap(service.Upstreams, service.Name))
	h := val.(*LeastConnectionsHeap)

	// If loaded from cache, we might need to refresh connection counts
	if loaded {
		h.refreshConnectionCounts(service.Name)
	}
	return h
}

// refreshConnectionCounts updates all items with current connection counts
func (h *LeastConnectionsHeap) refreshConnectionCounts(serviceName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, item := range h.items {
		conns := GetActiveConnections(serviceName, item.UpstreamId)
		if conns != item.ConnectionCount {
			item.ConnectionCount = conns
			item.Score = float64(conns+1) / item.Weight
		}
	}
	heap.Init(h) // Re-heapify after bulk updates
}

// ResetLeastConnHeapCache clears the heap cache (for testing)
func ResetLeastConnHeapCache() {
	leastConnHeapCache = sync.Map{}
}

// IncrementHeapConnection increments connection count in heap - O(log N)
func IncrementHeapConnection(serviceName, upstreamId string) {
	val, ok := leastConnHeapCache.Load(serviceName)
	if !ok {
		return
	}
	h := val.(*LeastConnectionsHeap)
	h.UpdateConnection(upstreamId, 1)
}

// DecrementHeapConnection decrements connection count in heap - O(log N)
func DecrementHeapConnection(serviceName, upstreamId string) {
	val, ok := leastConnHeapCache.Load(serviceName)
	if !ok {
		return
	}
	h := val.(*LeastConnectionsHeap)
	h.UpdateConnection(upstreamId, -1)
}
