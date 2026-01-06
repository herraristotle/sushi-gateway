<a href="https://rawsashimi1604.github.io/sushi-gateway">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./docs/public/images/LogoWithText_Dark.png">
    <source media="(prefers-color-scheme: light)" srcset="./docs/public/images/LogoWithText_Light.png">
    <img 
      alt="Sushi Gateway Logo"
      src="./docs/public/images/LogoWithText_Light.png"
      width="450">
  </picture>
</a>

<br/>
<br/>

![Stars](https://img.shields.io/github/stars/rawsashimi1604/sushi-gateway?style=flat-square) ![GitHub commit activity](https://img.shields.io/github/commit-activity/m/rawsashimi1604/sushi-gateway?style=flat-square) ![Docker Pulls](https://img.shields.io/docker/pulls/rawsashimi/sushi-proxy?style=flat-square) ![Version](https://img.shields.io/github/v/release/rawsashimi1604/sushi-gateway?color=green&label=Version&style=flat-square) ![License](https://img.shields.io/badge/License-MIT-yellow?style=flat-square)

**Sushi Gateway** is a production-ready, lightweight API Gateway built in Go. It provides enterprise-grade features like distributed rate limiting, response caching, BFF pattern support, and Prometheus metrics—comparable to Kong, KrakenD, and Tyk.

---

[Installation Guide](https://rawsashimi1604.github.io/sushi-gateway/getting-started/docker.html) | [Documentation](https://rawsashimi1604.github.io/sushi-gateway/docs-home.html) | [Releases](https://github.com/rawsashimi1604/sushi-gateway/releases)

---

## ✨ Key Features

| Category | Features |
|----------|----------|
| **Routing** | Dynamic paths, header-based routing, canary deployments |
| **Load Balancing** | Round-robin, Least-connections, Consistent-hashing, Latency EWMA |
| **Rate Limiting** | Distributed (Redis), per-second/minute/hour, X-RateLimit headers |
| **Caching** | Response caching with Redis, Cache-Control support, X-Cache headers |
| **BFF Pattern** | Response aggregation from multiple backends in parallel |
| **Security** | mTLS, JWT, API Key, Basic Auth, RBAC (Casbin), Bot Protection |
| **Resilience** | Circuit breaker (gobreaker), retry with exponential backoff |
| **Observability** | OpenTelemetry, Prometheus `/metrics`, JSON structured logging |

## 🚀 Quick Start

### Using Docker Compose

```bash
# Clone the repository
git clone https://github.com/rawsashimi1604/sushi-gateway.git
cd sushi-gateway

# Start Redis + Gateway + Test Services
docker compose -f docker-compose.e2e.yml up --build
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `CONFIG_FILE_PATH` | ✅ | Path to gateway config (YAML/JSON) |
| `ADMIN_USER` | ✅ | Admin API username |
| `ADMIN_PASSWORD` | ✅ | Admin API password |
| `REDIS_ADDR` | ✅ | Redis address (e.g., `localhost:6379`) |
| `REDIS_PASSWORD` | ❌ | Redis password (optional) |

### Ports

| Port | Purpose |
|------|---------|
| `8080` | HTTP Proxy |
| `8443` | HTTPS Proxy |
| `8081` | Admin API + Prometheus Metrics |

### Admin API Endpoints

| Endpoint | Description |
|----------|-------------|
| `/api/gateway` | Gateway configuration |
| `/api/health` | Upstream health status |
| `/api/stats` | Load balancing metrics |
| `/metrics` | Prometheus metrics |

## 📦 Plugins

| Plugin | Description |
|--------|-------------|
| `rate_limit` | Distributed rate limiting with Redis |
| `cache` | Response caching with TTL |
| `jwt` | JWT token validation |
| `basic_auth` | Basic authentication |
| `key_auth` | API key authentication |
| `acl` | IP whitelist/blacklist |
| `cors` | Cross-origin resource sharing |
| `circuit_breaker` | Fault tolerance with gobreaker |
| `rbac` | Role-based access control (Casbin) |
| `sanitization` | Request sanitization (XSS, SQL injection) |
| `bot_protection` | Block bad bots |
| `header_transformation` | Modify request/response headers |
| `http_log` | Send logs to HTTP endpoint |
| `mtls` | Mutual TLS for upstreams |

## 🔀 Response Aggregation (BFF Pattern)

Aggregate multiple backend responses into a single response:

```yaml
routes:
  - name: user-dashboard
    path: /v1/user/{id}
    methods: [GET]
    backends:
      - name: user
        host: user-service
        port: 3000
        path: /users/{id}
        required: true
      - name: orders
        host: order-service
        port: 3000
        path: /orders/{id}
        required: false
      - name: notifications
        host: notification-service
        port: 3000
        path: /notifications/{id}
        required: false
```

**Response:**
```json
{
  "data": {
    "user": { "name": "John" },
    "orders": [...],
    "notifications": { "count": 5 }
  },
  "_meta": {
    "total_backends": 3,
    "successful_calls": 3,
    "total_duration_ms": 45
  }
}
```

## 📊 Prometheus Metrics

Access metrics at `http://localhost:8081/metrics`:

```promql
# Request rate
rate(sushi_gateway_requests_total[5m])

# Cache hit ratio
rate(sushi_gateway_cache_hits_total[5m]) / 
(rate(sushi_gateway_cache_hits_total[5m]) + rate(sushi_gateway_cache_misses_total[5m]))

# Rate limit violations
rate(sushi_gateway_rate_limit_hits_total[5m])
```

## 🧪 E2E Testing

```bash
# Run E2E tests with Docker Compose
docker compose -f docker-compose.e2e.yml up --build

# View test results
docker compose -f docker-compose.e2e.yml logs e2e-tests
```

## 🏗️ Building from Source

```bash
# Requirements: Go 1.23+
cd sushi-proxy

# Build
go build -o sushi-proxy ./cmd

# Run
export REDIS_ADDR=localhost:6379
export ADMIN_USER=admin
export ADMIN_PASSWORD=secret
export CONFIG_FILE_PATH=config/config.yaml
./sushi-proxy

# Run tests
go test ./...
```

## 🖥️ Sushi Manager UI

Web-based UI for monitoring and management:

| Page | Features |
|------|----------|
| `/upstreams` | Real-time metrics, health badges, EWMA latency |
| `/health` | Auto-refresh health dashboard |
| `/services` | Upstream count, aggregated health |
| `/consumers` | Consumer management |

```bash
cd sushi-manager
npm install && npm run dev
```

## 🗺️ Roadmap

See the [Roadmap](ROADMAP.md) for future plans.

## 🤝 Contributing

We ❤️ contributions! Check out the [Contributing Guide](CONTRIBUTING.md).

- **Discussions**: [GitHub Discussions](https://github.com/rawsashimi1604/sushi-gateway/discussions)
- **Discord**: [Join our Discord](https://discord.gg/aPv4QhQ6)
- **Issues**: [GitHub Issues](https://github.com/rawsashimi1604/sushi-gateway/issues)

## 📄 License

MIT License - see [LICENSE](LICENSE)
