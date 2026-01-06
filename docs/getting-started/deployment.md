# Deployment Guide

This guide covers deploying Sushi Gateway in production environments.

## Architecture Overview

A production deployment typically consists of:

```
                  ┌─────────────────┐
                  │   Load Balancer │
                  └────────┬────────┘
                           │
                  ┌────────┴────────┐
                  │                 │
           ┌──────┴───────┐  ┌─────┴───────┐
           │ Sushi-Proxy  │  │Sushi-Proxy  │
           │   (8000)     │  │   (8000)    │
           └──────┬───────┘  └─────┬───────┘
                  │                │
           ┌──────┴────────────────┴──────┐
           │                               │
    ┌──────┴────────┐           ┌─────────┴────────┐
    │  Sushi-Manager│           │   Backend APIs   │
    │    (UI: 5173) │           │   (Upstreams)    │
    └───────────────┘           └──────────────────┘
```

---

## Docker Deployment

### Using Docker Compose

**docker-compose.yml:**
```yaml
version: '3.8'

services:
  sushi-proxy:
    image: rawsashimi/sushi-proxy:latest
    ports:
      - "8000:8000"  # Gateway port
      - "8001:8001"  # Admin API port
    volumes:
      - ./config.yaml:/app/config.yaml
    environment:
      - ADMIN_CORS_ORIGIN=http://localhost:5173
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8001/health"]
      interval: 10s
      timeout: 5s
      retries: 3
    restart: unless-stopped

  sushi-manager:
    build: ./sushi-manager
    ports:
      - "5173:5173"
    depends_on:
      - sushi-proxy
    environment:
      - VITE_API_URL=http://sushi-proxy:8001
    restart: unless-stopped
```

**Deploy:**
```bash
docker-compose up -d
```

---

## Kubernetes Deployment

### Helm Chart

**values.yaml:**
```yaml
replicaCount: 3

image:
  repository: rawsashimi/sushi-proxy
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: LoadBalancer
  port: 8000
  adminPort: 8001

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

config:
  yaml: |
    admin_cors_origin: "https://gateway-ui.example.com"
    services:
      - name: api-service
        # ... service configuration
```

**Deploy:**
```bash
helm install sushi-gateway ./helm/sushi-gateway \
  --values values.yaml \
  --namespace gateway \
  --create-namespace
```

---

## Configuration Management

### Environment Variables

```bash
# Gateway ports
GATEWAY_PORT=8000
ADMIN_PORT=8001

# CORS configuration
ADMIN_CORS_ORIGIN=https://gateway-ui.example.com

# Config file path
CONFIG_FILE_PATH=/app/config.yaml

# Log level
LOG_LEVEL=info
```

### Secrets Management

Use Kubernetes Secrets for sensitive data:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sushi-gateway-secrets
type: Opaque
stringData:
  jwt_secret: "your-secret-key"
  api_key: "your-api-key"
```

---

## Health Checks

### Liveness Probe

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8001
  initialDelaySeconds: 30
  periodSeconds: 10
  failureThreshold: 3
```

### Readiness Probe

```yaml
readinessProbe:
  httpGet:
    path: /health
    port: 8001
  initialDelaySeconds: 10
  periodSeconds: 5
  failureThreshold: 2
```

---

## Monitoring

### Prometheus Integration

**ServiceMonitor:**
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: sushi-gateway
spec:
  selector:
    matchLabels:
      app: sushi-proxy
  endpoints:
    - port: admin
      path: /metrics
      interval: 15s
```

### Key Metrics

Monitor these critical metrics:

| Metric | Alert Threshold | Action |
|--------|----------------|--------|
| `sushi_gateway_requests_total` | - | Track request volume |
| `sushi_gateway_request_duration_seconds` | p95 > 2s | Investigate slow upstreams |
| `sushi_gateway_upstream_health` | < 50% | Check upstream health |
| `sushi_gateway_rate_limit_hits_total` | Spike | Review rate limits |

---

## Performance Tuning

### Resource Allocation

**Recommended Resources:**

| Component | CPU | Memory | Replicas |
|-----------|-----|--------|----------|
| Sushi-Proxy | 500m-1000m | 512Mi-1Gi | 3+ |
| Sushi-Manager | 100m-250m | 128Mi-256Mi | 2 |

### Load Balancing Algorithm Selection

Choose based on workload:

```yaml
# High-traffic, stateless
load_balancing_strategy: round-robin

# Long-lived connections
load_balancing_strategy: least-connections

# Session affinity needed
load_balancing_strategy: consistent-hashing

# Performance-critical
load_balancing_strategy: latency
```

---

## High Availability

### Multiple Replicas

```yaml
replicaCount: 3  # Minimum for HA
```

### Pod Disruption Budget

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: sushi-proxy-pdb
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: sushi-proxy
```

### Anti-Affinity

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app: sushi-proxy
          topologyKey: kubernetes.io/hostname
```

---

## Security

### TLS Configuration

```yaml
services:
  - name: secure-api
    protocol: https
    tls:
      enabled: true
      cert_file: /certs/tls.crt
      key_file: /certs/tls.key
```

### Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: sushi-gateway-policy
spec:
  podSelector:
    matchLabels:
      app: sushi-proxy
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              role: frontend
      ports:
        - protocol: TCP
          port: 8000
```

---

## Backup & Recovery

### Configuration Backup

```bash
# Backup config
kubectl get configmap sushi-config -o yaml > backup/config-$(date +%Y%m%d).yaml

# Restore
kubectl apply -f backup/config-20260106.yaml
```

### Disaster Recovery

1. **Config stored in Git**: Version-controlled YAML
2. **Stateless design**: No data loss on pod restart
3. **Quick rollback**: Helm history & rollback

```bash
# Rollback to previous version
helm rollback sushi-gateway
```

---

## Scaling

### Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: sushi-proxy-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: sushi-proxy
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

---

## Troubleshooting

### Common Issues

#### 503 Service Unavailable

**Cause**: All upstreams unhealthy

**Check**:
```bash
curl http://localhost:8001/api/health
```

**Solution**: Fix upstream services or adjust health check thresholds

#### High Latency

**Cause**: Slow upstreams or overload

**Check**:
```bash
curl http://localhost:8001/api/stats | jq '.upstreams[] | {target, ewma_latency_ms}'
```

**Solution**: Use latency-based load balancing or add more upstreams

#### Rate Limit Spikes

**Cause**: Legitimate traffic spike or attack

**Check**:
```bash
curl http://localhost:8001/api/stats | jq '.rate_limits'
```

**Solution**: Adjust rate limits or implement additional bot protection

---

## Production Checklist

- [ ] Multiple replicas (3+ for HA)
- [ ] Resource limits configured
- [ ] Health checks enabled
- [ ] Prometheus monitoring configured
- [ ] Alerting rules set up
- [ ] TLS enabled for production traffic
- [ ] Backup strategy in place
- [ ] Disaster recovery tested
- [ ] Network policies applied
- [ ] Autoscaling configured

---

## Next Steps

- [Load Balancing Guide](../concepts/load-balancing.md)
- [Health Checks](../concepts/health.md)
- [Admin API Reference](../api/admin-api.md)
- [UI Guide](./ui-guide.md)
