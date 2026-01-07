# Circuit Breaker Plugin

The Circuit Breaker (`circuit-breaker`) plugin provides fault tolerance by preventing cascading failures when upstream services are unhealthy. It uses the [gobreaker](https://github.com/sony/gobreaker) library implementing the circuit breaker pattern.

## How It Works

The circuit breaker monitors upstream responses and tracks consecutive failures. When failures exceed a threshold, the circuit "opens" and immediately rejects requests without contacting the upstream. After a timeout, it enters "half-open" state to test if the upstream has recovered.

```
┌────────┐   failures >= threshold   ┌────────┐
│ CLOSED ├──────────────────────────►│  OPEN  │
└────┬───┘                           └────┬───┘
     │                                    │ timeout
     │ request succeeds                   ▼
     │                             ┌────────────┐
     └─────────────────────────────┤ HALF-OPEN  │
           success_threshold met   └────────────┘
```

### States

| State | Behavior |
|-------|----------|
| **Closed** | Requests pass through normally |
| **Open** | Requests immediately fail with 503 |
| **Half-Open** | Limited requests test upstream health |

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `failure_threshold` | Integer | ✅ | - | Consecutive failures before opening circuit |
| `success_threshold` | Integer | ❌ | 1 | Successes in half-open to close circuit |
| `timeout` | Integer | ✅ | - | Seconds before attempting half-open |

## Example Configuration

### Minimal Configuration

```yaml
plugins:
  - name: circuit-breaker
    config:
      failure_threshold: 5
      timeout: 30
```

### Production Configuration

```yaml
plugins:
  - name: circuit-breaker
    config:
      failure_threshold: 5   # Open after 5 consecutive failures
      success_threshold: 2   # Require 2 successes to close
      timeout: 60            # Wait 60s before testing
```

## Response When Open

When the circuit is open, clients receive:

```json
{
  "error": "SERVICE_UNAVAILABLE",
  "message": "Service temporarily unavailable (circuit breaker open)"
}
```

HTTP Status: `503 Service Unavailable`

## Failure Detection

The following responses are counted as failures:
- HTTP status codes 500-599
- Connection timeouts
- Connection refused

## Use Cases

1. **Upstream Protection**: Prevent overwhelming a struggling upstream
2. **Fail Fast**: Return errors quickly instead of waiting for timeouts
3. **Graceful Degradation**: Allow upstream recovery time

## Monitoring

Monitor circuit breaker state via logs:

```
Circuit breaker state change service=api-service from=closed to=open
Circuit breaker tripping service=api-service failures=5
```

## Best Practices

::: tip
Use with health checks for complete resilience:
- Health checks detect unhealthy upstreams proactively
- Circuit breaker handles runtime failures
:::

::: warning
Set `failure_threshold` high enough to avoid false positives from transient errors.
:::

## Related Concepts

- [Health Checks](../concepts/health.md) - Proactive upstream health monitoring
- [Load Balancing](../concepts/load-balancing.md) - Distribute load across healthy upstreams

For more plugins, visit the **[Plugins Overview](./index.md)**.
