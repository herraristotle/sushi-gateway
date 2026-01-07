# Prometheus Plugin

The Prometheus (`prometheus`) plugin exposes metrics for monitoring and alerting via the `/metrics` endpoint on the Admin API port.

## How It Works

When enabled, the plugin collects request metrics and exposes them in Prometheus format at `http://localhost:8081/metrics`. These metrics can be scraped by Prometheus and visualized in Grafana.

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `status_code_metrics` | Boolean | ❌ | true | Collect per-status-code metrics |
| `latency_metrics` | Boolean | ❌ | true | Collect request latency metrics |
| `bandwidth_metrics` | Boolean | ❌ | true | Collect request/response size metrics |
| `upstream_health_metrics` | Boolean | ❌ | true | Expose upstream health status |
| `per_consumer` | Boolean | ❌ | true | Break down metrics by consumer |

## Example Configuration

```yaml
plugins:
  - name: prometheus
    config:
      status_code_metrics: true
      latency_metrics: true
      bandwidth_metrics: true
      upstream_health_metrics: true
      per_consumer: true
```

## Available Metrics

### Request Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `sushi_gateway_requests_total` | Counter | Total HTTP requests |
| `sushi_gateway_request_duration_seconds` | Histogram | Request latency distribution |
| `sushi_gateway_requests_size_bytes` | Histogram | Request body sizes |
| `sushi_gateway_responses_size_bytes` | Histogram | Response body sizes |

### Cache Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `sushi_gateway_cache_hits_total` | Counter | Cache hit count |
| `sushi_gateway_cache_misses_total` | Counter | Cache miss count |

### Rate Limit Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `sushi_gateway_rate_limit_hits_total` | Counter | Rate-limited requests |

### Upstream Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `sushi_gateway_upstream_requests_total` | Counter | Requests to each upstream |
| `sushi_gateway_upstream_health` | Gauge | Upstream health status (1=healthy) |

## Prometheus Scrape Configuration

Add to `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'sushi-gateway'
    static_configs:
      - targets: ['sushi-proxy:8081']
    metrics_path: /metrics
    scrape_interval: 15s
```

## Example Queries

### Request Rate

```promql
rate(sushi_gateway_requests_total[5m])
```

### Error Rate

```promql
sum(rate(sushi_gateway_requests_total{status=~"5.."}[5m])) /
sum(rate(sushi_gateway_requests_total[5m]))
```

### P95 Latency

```promql
histogram_quantile(0.95, 
  rate(sushi_gateway_request_duration_seconds_bucket[5m])
)
```

### Cache Hit Ratio

```promql
rate(sushi_gateway_cache_hits_total[5m]) /
(rate(sushi_gateway_cache_hits_total[5m]) + rate(sushi_gateway_cache_misses_total[5m]))
```

## Grafana Dashboard

Import the recommended dashboard or create panels for:

- Request rate and error rate
- Latency percentiles (P50, P95, P99)
- Cache hit ratio
- Rate limit violations
- Upstream health status

## Alerting Rules

Example Prometheus alerting rules:

```yaml
groups:
  - name: sushi-gateway
    rules:
      - alert: HighErrorRate
        expr: |
          sum(rate(sushi_gateway_requests_total{status=~"5.."}[5m])) /
          sum(rate(sushi_gateway_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
      
      - alert: UpstreamUnhealthy
        expr: sushi_gateway_upstream_health == 0
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Upstream {{ $labels.target }} is unhealthy"
```

## Use Cases

1. **Performance Monitoring**: Track latency and throughput
2. **Capacity Planning**: Understand traffic patterns
3. **Alerting**: Detect errors and performance degradation
4. **Debugging**: Identify slow upstreams

## Related Plugins

- [Cache Plugin](./cache.md) - Metrics for cache hits/misses
- [Rate Limit Plugin](./rate-limit.md) - Rate limit hit metrics
- [OpenTelemetry Plugin](./opentelemetry.md) - Distributed tracing

For more plugins, visit the **[Plugins Overview](./index.md)**.
