# Troubleshooting Guide

This guide covers common issues encountered when using Sushi Gateway and their solutions.

## Quick Diagnostics

Before troubleshooting, gather information:

```bash
# Check gateway health
curl http://localhost:8081/api/health

# Check gateway configuration
curl http://localhost:8081/api/gateway | jq

# Check load balancing stats
curl http://localhost:8081/api/stats | jq

# Check Prometheus metrics
curl http://localhost:8081/metrics
```

---

## Common Issues

### 503 Service Unavailable

**Symptoms**: All requests return 503 errors.

**Causes**:
1. All upstreams are unhealthy
2. Circuit breaker is open
3. No upstreams configured

**Diagnosis**:
```bash
curl http://localhost:8081/api/health
```

**Solutions**:

| Cause | Solution |
|-------|----------|
| Upstreams unhealthy | Fix upstream services or adjust health check thresholds |
| Circuit breaker open | Wait for timeout or fix upstream |
| No upstreams | Check configuration for upstream targets |

---

### 429 Too Many Requests

**Symptoms**: Requests are rate limited.

**Causes**:
1. Legitimate rate limit reached
2. Rate limit misconfiguration
3. Redis connection issues (distributed mode)

**Diagnosis**:
```bash
curl http://localhost:8081/api/stats | jq '.rate_limits'
```

**Solutions**:
- Increase rate limits in configuration
- Check Redis connectivity for distributed rate limiting
- Implement client-side request pacing

---

### 401 Unauthorized

**Symptoms**: Authentication failing.

**Causes**:
1. Missing or invalid JWT token
2. Invalid API key
3. Incorrect authentication configuration

**Diagnosis**:
Check the plugin configuration and token validity.

**Solutions**:

| Plugin | Common Issues |
|--------|---------------|
| JWT | Token expired, wrong secret, invalid issuer |
| Key-Auth | Key not registered in consumers |
| Basic Auth | Wrong credentials |

---

### 403 Forbidden

**Symptoms**: Authorization failing.

**Causes**:
1. ACL blocking IP
2. RBAC denying access
3. Consumer not in allowed group

**Solutions**:
- Check ACL whitelist/blacklist
- Verify RBAC policies
- Check consumer group assignments

---

### High Latency

**Symptoms**: Requests are slow.

**Causes**:
1. Slow upstream services
2. Inefficient load balancing
3. Redis latency (caching/rate limiting)
4. Too many plugins

**Diagnosis**:
```bash
# Check upstream latency
curl http://localhost:8081/api/stats | jq '.upstreams[] | {target, ewma_latency_ms}'
```

**Solutions**:
- Use latency-based load balancing
- Enable caching for frequently accessed data
- Reduce plugin chain length
- Add more upstream capacity

---

### Connection Refused

**Symptoms**: Gateway cannot connect to upstreams.

**Causes**:
1. Upstream service not running
2. Wrong host/port configuration
3. Network/firewall issues
4. DNS resolution failure

**Diagnosis**:
```bash
# From Gateway container
curl -v http://upstream-host:port/health
```

**Solutions**:
- Verify upstream is running and accessible
- Check Docker/Kubernetes networking
- Verify DNS resolution
- Check firewall rules

---

### Plugin Configuration Errors

**Symptoms**: Gateway fails to start or plugin doesn't work.

**Diagnosis**:
Check gateway startup logs for validation errors.

**Common Errors**:

| Error | Solution |
|-------|----------|
| `ttl must be greater than 0` | Cache TTL must be positive |
| `failure_threshold must be positive` | Circuit breaker needs valid threshold |
| `at least one operation must be configured` | Header transformation needs add/remove/etc |

---

## Debug Mode

Enable debug logging:

```yaml
# In configuration
log_level: debug
```

Or via environment variable:
```bash
export LOG_LEVEL=debug
```

---

## Health Check Verification

Verify health checks are working:

```bash
# Check all upstream health
curl http://localhost:8081/api/health | jq

# Expected healthy response
{
  "upstream_id": "...",
  "status": "healthy",
  "successes": 10,
  "failures": 0
}
```

---

## Plugin Conflict Resolution

Some plugins may conflict:

| Conflict | Resolution |
|----------|------------|
| Multiple auth plugins | Use `multi-auth` to combine |
| Rate limit + Cache | Cached responses bypass rate limit |
| RBAC + no auth | RBAC requires prior authentication |

---

## Performance Debugging

### CPU Usage High

**Diagnosis**:
```bash
# Check Prometheus metrics
curl http://localhost:8081/metrics | grep cpu
```

**Solutions**:
- Reduce plugin count
- Increase replicas
- Check for inefficient regex in routing

### Memory Usage High

**Diagnosis**:
```bash
# Check memory metrics
curl http://localhost:8081/metrics | grep memory
```

**Solutions**:
- Reduce cache size
- Check for memory leaks in custom plugins
- Increase container memory limits

---

## Getting Help

If issues persist:

1. Check [GitHub Issues](https://github.com/rawsashimi1604/sushi-gateway/issues)
2. Join [Discord](https://discord.gg/aPv4QhQ6)
3. Create detailed bug report with:
   - Configuration (redacted secrets)
   - Error messages
   - Steps to reproduce
