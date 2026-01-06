# Admin API Reference

The Sushi Gateway Admin API provides endpoints for monitoring and managing the gateway. All endpoints are available at `http://localhost:8001/api` by default.

## Authentication

Currently, the Admin API does not require authentication. In production, ensure this port is properly secured.

## Health Status

### GET /api/health

Returns health status for all upstreams across all services.

**Response:**
```json
[
  {
    "upstream_id": "uuid-string",
    "service_name": "api-service",
    "target": "localhost:8080",
    "status": "healthy",
    "check_type": "active",
    "last_checked": "2026-01-06T05:30:00Z",
    "successes": 150,
    "failures": 0
  }
]
```

**Fields:**
- `upstream_id` (string): Unique identifier for the upstream
- `service_name` (string): Name of the service this upstream belongs to
- `target` (string): Host:port of the upstream server
- `status` (string): Health status - `healthy`, `unhealthy`, or `not_available`
- `check_type` (string): Type of health check - `active`, `passive`, `both`, or `unknown`
- `last_checked` (string): ISO 8601 timestamp of last health check
- `successes` (number): Consecutive successful health checks
- `failures` (number): Consecutive failed health checks

---

## Load Balancing Stats

### GET /api/stats

Returns comprehensive load balancing statistics including upstream metrics and rate limiting data.

**Response:**
```json
{
  "upstreams": [
    {
      "upstream_id": "uuid-string",
      "service_name": "api-service",
      "target": "localhost:8080",
      "weight": 100,
      "active_connections": 5,
      "ewma_latency_ms": 12.45,
      "health_status": "healthy"
    }
  ],
  "rate_limits": [
    {
      "scope": "global",
      "hits": 42,
      "allowed": 9958,
      "hit_rate": 0.42
    }
  ]
}
```

**Upstream Fields:**
- `upstream_id` (string): Unique identifier
- `service_name` (string): Parent service name
- `target` (string): Upstream host:port
- `weight` (number): Load balancing weight (1-1000)
- `active_connections` (number): Current active connections to this upstream
- `ewma_latency_ms` (number): Exponentially weighted moving average latency in milliseconds
- `health_status` (string): Current health status

**Rate Limit Fields:**
- `scope` (string): Rate limit scope (e.g., `global`, `service:name`)
- `hits` (number): Total requests that hit the rate limit
- `allowed` (number): Total requests allowed through
- `hit_rate` (number): Percentage of requests that were rate limited

---

## Gateway Configuration

### GET /api/gateway

Returns the complete gateway configuration including services, routes, plugins, and upstreams.

**Response:**
```json
{
  "gateway": {
    "name": "sushi-gateway",
    "global": { /* global config */ },
    "services": [
      {
        "name": "api-service",
        "base_path": "/api",
        "protocol": "http",
        "load_balancing_strategy": "round-robin",
        "upstreams": [
          {
            "id": "uuid",
            "target": "localhost:8080",
            "weight": 100
          }
        ],
        "routes": [ /* routes */ ],
        "plugins": [ /* plugins */ ]
      }
    ]
  },
  "config": {
    "port": 8000,
    "admin_port": 8001
  }
}
```

---

## Prometheus Metrics

### GET /metrics

Prometheus-compatible metrics endpoint for monitoring and alerting.

**Metrics Available:**
- `sushi_gateway_requests_total` - Total HTTP requests
- `sushi_gateway_request_duration_seconds` - Request latency histogram
- `sushi_gateway_rate_limit_hits_total` - Rate limit hits counter
- `sushi_gateway_upstream_requests_total` - Upstream request counter
- And more...

See the [Monitoring Guide](../concepts/monitoring.md) for complete metric documentation.

---

## Auto-Refresh Recommendations

For UI applications consuming these APIs:

| Endpoint | Recommended Refresh | Use Case |
|----------|---------------------|----------|
| `/api/health` | 5 seconds | Real-time health monitoring |
| `/api/stats` | 10 seconds | Load balancing metrics |
| `/api/gateway` | 30 seconds | Configuration changes |
| `/metrics` | 15-30 seconds | Prometheus scraping |

---

## Error Responses

All endpoints return standard HTTP status codes:

- `200 OK` - Success
- `500 Internal Server Error` - Gateway configuration not loaded or internal error

**Error Response Format:**
```json
{
  "error": "Gateway config not loaded"
}
```

---

## CORS Configuration

The Admin API supports CORS for frontend access. Configure via:

```yaml
admin_cors_origin: "http://localhost:5173"
```

Default origin is `http://localhost:5173` for local development.
