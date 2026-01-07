# Monitoring Guide

This guide covers setting up monitoring and observability for Sushi Gateway using Prometheus and Grafana.

## Overview

Sushi Gateway exposes metrics at `http://localhost:8081/metrics` in Prometheus format.

## Quick Setup

### 1. Enable Prometheus Plugin

```yaml
plugins:
  - name: prometheus
    config:
      status_code_metrics: true
      latency_metrics: true
      bandwidth_metrics: true
      upstream_health_metrics: true
```

### 2. Configure Prometheus

Add to `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'sushi-gateway'
    static_configs:
      - targets: ['sushi-proxy:8081']
    metrics_path: /metrics
    scrape_interval: 15s
```

## Key Metrics

### Request Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `sushi_gateway_requests_total` | Counter | Total requests by status/method |
| `sushi_gateway_request_duration_seconds` | Histogram | Request latency |

### Upstream Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `sushi_gateway_upstream_health` | Gauge | 1=healthy, 0=unhealthy |
| `sushi_gateway_upstream_requests_total` | Counter | Requests per upstream |

### Cache Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `sushi_gateway_cache_hits_total` | Counter | Cache hit count |
| `sushi_gateway_cache_misses_total` | Counter | Cache miss count |

## Alerting Rules

Example Prometheus alerts:

```yaml
groups:
  - name: sushi-gateway
    rules:
      - alert: GatewayHighErrorRate
        expr: |
          sum(rate(sushi_gateway_requests_total{status=~"5.."}[5m])) /
          sum(rate(sushi_gateway_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate (> 5%)"
      
      - alert: UpstreamUnhealthy
        expr: sushi_gateway_upstream_health == 0
        for: 1m
        labels:
          severity: warning
```

## Grafana Dashboards

Import the Sushi Gateway dashboard or create panels for:

- Request rate and error rate
- P50/P95/P99 latency
- Upstream health status
- Cache hit ratio
- Rate limit hits

## Health Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/api/health` | Upstream health status |
| `/api/stats` | Load balancing statistics |
| `/metrics` | Prometheus metrics |

::: tip
Auto-refresh recommendations:
- `/api/health`: 5 seconds
- `/api/stats`: 10 seconds
- `/metrics`: 15-30 seconds
:::

## Related Documentation

- [Prometheus Plugin](../plugins/prometheus.md)
- [OpenTelemetry Plugin](../plugins/opentelemetry.md)
- [Admin API Reference](../api/admin-api.md)
