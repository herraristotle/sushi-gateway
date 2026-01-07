# Complete Configuration Reference

This document provides a complete reference for all Sushi Gateway configuration options.

## Configuration File Format

Sushi Gateway supports YAML and JSON configuration formats.

::: tip
YAML is recommended for readability. JSON is useful for programmatic generation.
:::

## Global Configuration

```yaml
# Gateway name (required)
name: sushi-gateway

# Global plugins applied to all routes
plugins: []

# Service definitions
services: []

# Consumer definitions (for authentication)
consumers: []

# Upstream definitions (for load balancing)
upstreams: []
```

## Services

```yaml
services:
  - name: my-service          # Required: unique identifier
    url: http://upstream:8080 # Required: upstream base URL
    protocol: http            # Optional: http or https (default: http)
    connect_timeout: 5000     # Optional: connection timeout in ms
    read_timeout: 60000       # Optional: read timeout in ms
    write_timeout: 60000      # Optional: write timeout in ms
    retries: 3                # Optional: retry attempts on failure
    routes: []                # Required: route definitions
    plugins: []               # Optional: service-level plugins
```

## Routes

```yaml
routes:
  - name: my-route            # Required: unique identifier
    paths:                    # Required: path patterns
      - /api/v1/users
      - /api/v1/users/*
    methods:                  # Optional: allowed methods (default: all)
      - GET
      - POST
    strip_path: false         # Optional: strip matched path prefix
    preserve_host: true       # Optional: preserve original Host header
    plugins: []               # Optional: route-level plugins
```

## Consumers

```yaml
consumers:
  - username: my-user         # Required: unique identifier
    jwt_secrets:              # Optional: JWT credentials
      - algorithm: HS256
        key: issuer-key
        secret: base64secret
    keyauth_credentials:      # Optional: API key credentials
      - key: my-api-key
```

## Upstreams

```yaml
upstreams:
  - name: my-upstream         # Required: unique identifier
    algorithm: round-robin    # Optional: load balancing algorithm
    healthchecks:             # Optional: health check configuration
      active:
        healthy:
          interval: 5         # Check interval in seconds
          successes: 2        # Successes to mark healthy
        unhealthy:
          interval: 5
          http_failures: 3    # Failures to mark unhealthy
        http_path: /health    # Health check path
        type: http            # Check type: http or tcp
    targets:                  # Required: upstream targets
      - target: host:port
        weight: 100           # Load balancing weight
        tags:                 # Optional: for subset load balancing
          - name: version
            value: v1
```

## Load Balancing Algorithms

| Algorithm | Value | Description |
|-----------|-------|-------------|
| Round Robin | `round-robin` | Distribute evenly |
| Least Connections | `least-connections` | Route to least loaded |
| Consistent Hash | `consistent-hashing` | Session affinity |
| Latency | `latency` | Route to fastest |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CONFIG_FILE_PATH` | ✅ | - | Path to config file |
| `ADMIN_USER` | ✅ | - | Admin API username |
| `ADMIN_PASSWORD` | ✅ | - | Admin API password |
| `REDIS_ADDR` | ✅ | - | Redis address for rate limiting/cache |
| `REDIS_PASSWORD` | ❌ | - | Redis password |
| `LOG_LEVEL` | ❌ | `info` | Log level: debug, info, warn, error |
| `ADMIN_CORS_ORIGIN` | ❌ | `localhost:5173` | CORS origin for Admin API |

## Plugin Configuration

See [Plugins Overview](./plugins/index.md) for complete plugin configuration reference.

### Common Plugin Fields

```yaml
plugins:
  - name: plugin-name         # Required: plugin identifier
    enabled: true             # Optional: enable/disable (default: true)
    config:                   # Plugin-specific configuration
      key: value
```

## Complete Example

```yaml
name: production-gateway

plugins:
  - name: cors
    config:
      origins: ["https://example.com"]
      methods: [GET, POST, PUT, DELETE]
  
  - name: rate-limiting
    config:
      minute: 100
      hour: 5000

consumers:
  - username: api-user
    keyauth_credentials:
      - key: secret-api-key

services:
  - name: user-service
    url: http://users-upstream
    routes:
      - name: users-api
        paths: [/api/users, /api/users/*]
        methods: [GET, POST]

upstreams:
  - name: users-upstream
    algorithm: round-robin
    healthchecks:
      active:
        healthy:
          interval: 5
          successes: 2
        unhealthy:
          interval: 5
          http_failures: 3
        http_path: /health
        type: http
    targets:
      - target: user-service-1:8080
        weight: 100
      - target: user-service-2:8080
        weight: 100
```

## Related Documentation

- [Getting Started](./getting-started/deployment.md)
- [Plugins Overview](./plugins/index.md)
- [Load Balancing](./concepts/load-balancing.md)
