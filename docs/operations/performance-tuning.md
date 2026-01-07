# Performance Tuning Guide

This runbook covers performance optimization for Sushi Gateway.

## Performance Benchmarks

Baseline performance on modern hardware (4 CPU, 8GB RAM):

| Metric | Target | Typical |
|--------|--------|---------|
| Requests/sec | 50,000+ | 60,000 |
| P50 latency | < 5ms | 2ms |
| P95 latency | < 20ms | 8ms |
| P99 latency | < 50ms | 15ms |

## Quick Wins

### 1. Enable Response Caching

```yaml
plugins:
  - name: cache
    config:
      ttl: 300  # 5 minutes
```

**Impact**: 10-100x improvement for cached endpoints

### 2. Use Latency-Based Load Balancing

```yaml
upstreams:
  - name: my-upstream
    algorithm: latency
```

**Impact**: Routes to fastest upstream automatically

### 3. Reduce Plugin Chain

Remove unnecessary plugins. Each plugin adds ~0.1-1ms latency.

## Resource Tuning

### CPU Allocation

```yaml
resources:
  requests:
    cpu: 500m
  limits:
    cpu: 1000m
```

::: tip
1 CPU core handles ~10,000 RPS. Scale horizontally for higher throughput.
:::

### Memory Allocation

```yaml
resources:
  requests:
    memory: 256Mi
  limits:
    memory: 512Mi
```

### Replica Scaling

```yaml
# HPA configuration
spec:
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70
```

## Load Balancing Optimization

| Algorithm | Best For | Overhead |
|-----------|----------|----------|
| `round-robin` | General purpose | Lowest |
| `least-connections` | Long requests | Low |
| `latency` | Mixed workloads | Medium |
| `consistent-hashing` | Session affinity | Medium |

## Connection Tuning

```yaml
services:
  - name: my-service
    connect_timeout: 3000   # 3s connection timeout
    read_timeout: 30000     # 30s read timeout
    write_timeout: 30000    # 30s write timeout
    retries: 2              # Retry failed requests
```

## Health Check Optimization

```yaml
healthchecks:
  active:
    healthy:
      interval: 5       # Check every 5s
      successes: 2      # 2 successes to mark healthy
    unhealthy:
      interval: 2       # Check every 2s when unhealthy
      http_failures: 3  # 3 failures to mark unhealthy
```

## Monitoring Performance

### Key Metrics to Watch

```promql
# Request rate
rate(sushi_gateway_requests_total[5m])

# P95 latency
histogram_quantile(0.95, rate(sushi_gateway_request_duration_seconds_bucket[5m]))

# Error rate
sum(rate(sushi_gateway_requests_total{status=~"5.."}[5m])) /
sum(rate(sushi_gateway_requests_total[5m]))

# Cache hit ratio
rate(sushi_gateway_cache_hits_total[5m]) /
(rate(sushi_gateway_cache_hits_total[5m]) + rate(sushi_gateway_cache_misses_total[5m]))
```

## Troubleshooting High Latency

### Step 1: Identify Bottleneck

```bash
# Check upstream latency
curl http://localhost:8081/api/stats | jq '.upstreams[] | {target, ewma_latency_ms}'
```

### Step 2: Common Causes

| Symptom | Cause | Fix |
|---------|-------|-----|
| All upstreams slow | Upstream issue | Fix upstream services |
| One upstream slow | Uneven load | Use latency-based LB |
| High gateway CPU | Too many plugins | Remove unnecessary plugins |
| High memory | Large responses | Enable streaming |

### Step 3: Load Testing

```bash
# Using hey
hey -n 10000 -c 100 http://gateway:8080/api/endpoint

# Using vegeta
echo "GET http://gateway:8080/api/endpoint" | vegeta attack -rate=1000 -duration=30s | vegeta report
```

## Checklist

- [ ] Caching enabled for read endpoints
- [ ] Appropriate load balancing algorithm selected
- [ ] Health checks tuned for workload
- [ ] Resource limits set appropriately
- [ ] HPA configured for auto-scaling
- [ ] Monitoring dashboards configured

## Related

- [Monitoring Guide](./monitoring.md)
- [Cache Plugin](../plugins/cache.md)
- [Load Balancing](../concepts/load-balancing.md)
