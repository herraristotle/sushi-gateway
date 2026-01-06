# Load Balancing Algorithms

Sushi Gateway supports multiple load balancing algorithms with full Kong parity, each optimized for different use cases.

## Overview

| Algorithm | Use Case | Complexity | Kong Parity |
|-----------|----------|------------|-------------|
| Round Robin | General purpose | O(1) | ✅ |
| Least Connections | Connection-heavy workloads | O(log N) | ✅ |
| Consistent Hashing | Session affinity | O(log N) | ✅ |
| Latency-based EWMA | Performance-critical | O(N) | ✅ |

---

## Round Robin

Distributes requests evenly across all healthy upstreams in a circular manner.

```yaml
services:
  - name: api-service
    load_balancing_strategy: round-robin
    upstreams:
      - target: localhost:8080
        weight: 100
      - target: localhost:8081
        weight: 200  # Gets 2x more requests
```

**Weight Support**: Upstreams with higher weights receive proportionally more traffic.

---

## Least Connections

Routes to the upstream with fewest active connections.

```yaml
services:
  - name: api-service
    load_balancing_strategy: least-connections
    upstreams:
      - target: localhost:8080
        weight: 100
```

**Implementation**:
- Binary min-heap for 10+ upstreams (O(log N))
- Linear scan for <10 upstreams (O(N))
- Score: `(connections + 1) / weight`

**Best For**: Long-lived connections, WebSocket, streaming

---

## Consistent Hashing

Routes based on hash key for session affinity.

```yaml
services:
  - name: api-service
    load_balancing_strategy: consistent-hashing
    upstreams:
      - target: localhost:8080
        hash_on: consumer        # Hash source
        hash_on_cookie: session_id
        hash_on_header: X-User-ID
```

**Hash Sources** (priority order):
1. Consumer ID
2. Header value
3. Cookie value
4. Path
5. Query parameter
6. IP address (default)

---

## Latency-based EWMA

Routes to fastest upstream based on exponentially weighted moving average.

```yaml
services:
  - name: api-service
    load_balancing_strategy: latency
    upstreams:
      - target: localhost:8080
        weight: 100
```

**Features**:
- EWMA decay: 0.2 (20% new, 80% historical)
- Slow-start protection for new upstreams
- Weight-aware scoring: `EWMA / weight`

**Best For**: Performance-critical apps, heterogeneous upstreams

---

## Retry Tracking

All algorithms automatically avoid failed upstreams during retries.

```yaml
services:
  - name: api-service
    retry_attempts: 3
```

---

## Monitoring

Track algorithm performance:
- **Admin API**: `/api/stats` for real-time metrics
- **UI**: `/upstreams` and `/health` pages
- **Prometheus**: `/metrics` endpoint

**Key Metrics**:
- `active_connections`
- `ewma_latency_ms`
- `health_status`

---

## Best Practices

1. Start with `round-robin`
2. Use weights for capacity differences
3. Enable health checks for failover
4. Monitor distribution via `/api/stats`
5. Test failover scenarios

See the [complete load balancing guide](./load-balancing-detailed.md) for advanced configuration.
