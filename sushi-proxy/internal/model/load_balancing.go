package model

type LoadBalancingAlgorithm string

const (
	RoundRobin           LoadBalancingAlgorithm = "round-robin"
	Weighted             LoadBalancingAlgorithm = "weighted"
	IPHash               LoadBalancingAlgorithm = "ip-hash"
	LeastConnections     LoadBalancingAlgorithm = "least-connections"
	Latency              LoadBalancingAlgorithm = "latency"            // EWMA-based latency routing
	ConsistentHashing    LoadBalancingAlgorithm = "consistent-hashing" // Kong-style hash-based routing
	NoUpstreamsAvailable int                    = -1
)

// HashOn options for consistent-hashing algorithm (Kong parity)
type HashOnType string

const (
	HashOnNone     HashOnType = "none"
	HashOnConsumer HashOnType = "consumer"
	HashOnIP       HashOnType = "ip"
	HashOnHeader   HashOnType = "header"
	HashOnCookie   HashOnType = "cookie"
	HashOnPath     HashOnType = "path"
	HashOnQueryArg HashOnType = "query_arg"
)

func (alg LoadBalancingAlgorithm) IsValid() bool {
	if alg == "" {
		return true // Default to RoundRobin
	}
	switch alg {
	case RoundRobin, Weighted, IPHash, LeastConnections, Latency, ConsistentHashing:
		return true
	default:
		return false
	}
}
