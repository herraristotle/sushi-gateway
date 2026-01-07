# Incident Response Runbook

This runbook provides procedures for responding to Sushi Gateway incidents.

## Severity Levels

| Level | Description | Response Time |
|-------|-------------|---------------|
| **P1** | Complete outage | Immediate |
| **P2** | Partial outage (>10% errors) | 15 minutes |
| **P3** | Degraded performance | 1 hour |
| **P4** | Minor issue | Next business day |

---

## P1: Complete Outage

### Symptoms
- All requests failing (503/502)
- No healthy upstreams
- Gateway not responding

### Immediate Actions

```bash
# 1. Check gateway pods
kubectl get pods -l app=sushi-proxy

# 2. Check recent events
kubectl get events --sort-by='.lastTimestamp' | tail -20

# 3. Check gateway logs
kubectl logs -l app=sushi-proxy --tail=100
```

### Triage Decision Tree

```
Gateway pods running?
├── No → Restart pods or check node resources
└── Yes → Check health endpoint
    ├── Health endpoint fails → Check gateway config
    └── Health endpoint OK → Check upstreams
        ├── All upstreams unhealthy → Fix upstream services
        └── Upstreams healthy → Check routing config
```

### Resolution Steps

1. **Restart gateway** (if config unchanged):
   ```bash
   kubectl rollout restart deployment/sushi-proxy
   ```

2. **Rollback config** (if recent change):
   ```bash
   kubectl rollout undo deployment/sushi-proxy
   ```

3. **Scale up** (if overloaded):
   ```bash
   kubectl scale deployment/sushi-proxy --replicas=5
   ```

---

## P2: High Error Rate

### Symptoms
- Error rate >10%
- Intermittent 5xx responses
- Some requests succeeding

### Diagnosis

```bash
# Check error distribution
curl http://localhost:8081/metrics | grep 'status="5'

# Check upstream health
curl http://localhost:8081/api/health | jq

# Check rate limiting
curl http://localhost:8081/api/stats | jq '.rate_limits'
```

### Common Causes & Fixes

| Cause | Fix |
|-------|-----|
| Unhealthy upstream | Remove or fix upstream |
| Circuit breaker open | Wait for timeout or fix upstream |
| Rate limit misconfiguration | Adjust limits |
| Memory exhaustion | Scale up or restart |

---

## P3: Performance Degradation

### Symptoms
- Latency increased (P95 > 2s)
- Response times slow
- No errors

### Diagnosis

```bash
# Check latency metrics
curl http://localhost:8081/api/stats | jq '.upstreams[] | {target, ewma_latency_ms}'

# Check cache hit rate
curl http://localhost:8081/metrics | grep cache

# Check resource usage
kubectl top pods -l app=sushi-proxy
```

### Resolution

1. Enable caching for slow endpoints
2. Switch to latency-based load balancing
3. Add more upstream capacity
4. Scale gateway replicas

---

## Post-Incident

### Required Actions

1. **Document timeline** - What happened, when, impact
2. **Root cause analysis** - Why did it happen
3. **Action items** - How to prevent recurrence
4. **Update runbooks** - Add new scenarios

### Incident Report Template

```markdown
## Incident: [Title]
**Date**: YYYY-MM-DD
**Duration**: X hours Y minutes
**Severity**: P1/P2/P3/P4

### Summary
Brief description of what happened.

### Timeline
- HH:MM - First alert
- HH:MM - Investigation started
- HH:MM - Root cause identified
- HH:MM - Fix deployed
- HH:MM - Service restored

### Root Cause
Technical explanation.

### Action Items
- [ ] Implement fix
- [ ] Add monitoring
- [ ] Update runbook
```

## Related

- [Troubleshooting Guide](../troubleshooting/common-issues.md)
- [Monitoring Guide](./monitoring.md)
