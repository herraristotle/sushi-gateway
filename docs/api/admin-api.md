# Admin REST API Reference

The Sushi Gateway Admin API provides a comprehensive interface for managing and monitoring the gateway. It allows administrators to retrieve gateway state, manage services/routes/upstreams (in DB mode), and view performance metrics. All endpoints are available at `http://localhost:8081` by default.

## Access & Authentication

The Admin API is hosted on **port 8081** (default). 

### Base URL
```
http://<gateway-host>:8081
```

### Authentication (JWT + Basic Auth)

1. **Login**: Authenticate via `POST /login` using **Basic Auth** (`Authorization: Basic ...`).
2. **Session**: Upon success, the API returns a `Set-Cookie: token=<jwt>` header.
3. **Subsequent Requests**: Browsers/Clients must include this `HttpOnly` cookie.

::: tip
In **DB-less mode**, the Admin API is **read-only** (except for `/config` reload). All POST/PUT/DELETE operations on entities will return `405 Method Not Allowed`.
:::

---

## Core Endpoints

### Gateway State
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/gateway` | Returns the complete runtime configuration (ProxyConfig) |
| `GET` | `/gateway/config` | Returns the environment/boot configuration (AppConfig) |
| `POST` | `/config` | Force a configuration reload from the source |

### Observability
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/health` | Current health status of all upstreams |
| `GET` | `/api/stats` | Real-time performance metrics and load balancing stats |
| `GET` | `/metrics` | Prometheus-format metrics for scraping |

---

## Entity Management (DB Mode Only)

### Services
Manage the backend services known to the gateway.

- `GET /services`: List all services
- `POST /services`: Create or update a service (Body: `Service` object)
- `DELETE /services/{name}`: Delete a service by name

### Routes
Manage routing rules for a specific service.

- `GET /services/{serviceName}/routes`: List routes for a service
- `POST /services/{serviceName}/routes`: Create/update a route
- `DELETE /services/{serviceName}/routes/{routeName}`: Delete a route

### Upstreams
Manage target upstream servers globally.

- `GET /upstreams`: List all upstream targets
- `POST /upstreams`: Create/update an upstream
- `DELETE /upstreams/{name}`: Delete an upstream

---

## Detailed Resource Formats

### Health Response (`GET /api/health`)
```json
[
  {
    "upstream_id": "api-v1-target-1",
    "service_name": "api-service",
    "target": "10.0.0.5:8080",
    "status": "healthy",
    "check_type": "active",
    "last_checked": "2026-01-06T05:30:00Z",
    "successes": 150,
    "failures": 0
  }
]
```

### Health Response Details

| Field | Description |
|-------|-------------|
| `upstream_id` | Unique identifier for the upstream |
| `service_name` | Name of the service this upstream belongs to |
| `target` | Host:port of the upstream server |
| `status` | Health status - `healthy`, `unhealthy`, or `unknown` |
| `check_type` | Type of health check - `active`, `passive`, or `both` |
| `successes` | Consecutive successful health checks |
| `failures` | Consecutive failed health checks |
| `last_checked` | ISO 8601 timestamp of last health check |

### Stats Response (`GET /api/stats`)
```json
{
  "upstreams": [
    {
      "upstream_id": "api-v1",
      "active_connections": 12,
      "ewma_latency_ms": 45.2,
      "health_status": "healthy"
    }
  ],
  "rate_limits": [
    {
      "scope": "service:api-service",
      "hits": 1500,
      "allowed": 8500
    }
  ]
}
```

---

## Related Documentation

- [Monitoring Guide](../operations/monitoring.md)
- [Prometheus Plugin](../plugins/prometheus.md)
- [OpenTelemetry Tracing](../plugins/opentelemetry.md)
