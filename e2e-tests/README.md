# Sushi Gateway E2E Testing

This directory contains end-to-end tests for validating all Sushi Gateway features using the existing `test-services/sushi-api`.

## Quick Start

```bash
# Run E2E tests from project root
docker compose -f docker-compose.e2e.yml up --build

# Run tests in background and stream logs
docker compose -f docker-compose.e2e.yml up --build -d
docker compose -f docker-compose.e2e.yml logs -f e2e-tests

# Clean up
docker compose -f docker-compose.e2e.yml down -v
```

## Test Coverage

| Feature | Test | Endpoint |
|---------|------|----------|
| Health Check | Admin `/health` returns 200 | `GET /health` |
| Prometheus Metrics | `sushi_gateway_*` metrics present | `GET /metrics` |
| Proxy Pass | Sushi data from upstream | `GET /sushi-service/v1/sushi` |
| Load Balancing | Multiple app_ids in responses | `GET /sushi-service/v1/sushi` |
| Rate Limiting | 429 after limit, X-RateLimit headers | `GET /sushi-service/v1/sushi` |
| Response Caching | X-Cache: HIT/MISS headers | `GET /sushi-service/v1/sushi` |
| BFF Aggregation | Multi-backend merge, _meta object | `GET /dashboard/v1/user/{id}` |
| Upstream Health | Proxied health check | `GET /healthz/upstream` |
| Admin Auth | Unauthenticated returns 401 | `GET /v1/gateway` |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Docker Network (sushi-e2e)                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────┐    ┌──────────────┐    ┌─────────────────────┐    │
│  │  Redis  │◀───│ Sushi Gateway│───▶│   sushi-svc-1 (70%) │    │
│  └─────────┘    │   :8080      │    └─────────────────────┘    │
│                 │   :8081      │───▶│   sushi-svc-2 (30%) │    │
│                 └──────┬───────┘    └─────────────────────┘    │
│                        │                                        │
│                        ├───────────▶ user-service (sushi-api)   │
│                        ├───────────▶ order-service (sushi-api)  │
│                        └───────────▶ notification-service       │
│                                                                 │
│  ┌─────────────┐                                                │
│  │  E2E Tests  │ ────────────────▶ Gateway APIs                 │
│  │   (curl)    │                                                │
│  └─────────────┘                                                │
└─────────────────────────────────────────────────────────────────┘
```

## Services

| Service | Image | Purpose |
|---------|-------|---------|
| `redis` | redis:7-alpine | Rate limiting & caching |
| `sushi-svc-1` | test-services/sushi-api | Primary upstream (weight: 70) |
| `sushi-svc-2` | test-services/sushi-api | Secondary upstream (weight: 30) |
| `user-service` | test-services/sushi-api | BFF backend (sushi data) |
| `order-service` | test-services/sushi-api | BFF backend (restaurant data) |
| `notification-service` | test-services/sushi-api | BFF backend (health) |
| `sushi-proxy` | sushi-proxy | Gateway under test |
| `e2e-tests` | curlimages/curl | Test runner |

## Configuration Files

- **Gateway Config**: `sushi-proxy/config/config.e2e.yaml`
- **Test Script**: `e2e-tests/run-tests.sh`
- **Compose File**: `docker-compose.e2e.yml`
