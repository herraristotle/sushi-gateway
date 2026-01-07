# Shadow Traffic Plugin

The Shadow Traffic (`shadow-traffic`) plugin mirrors live traffic to a secondary endpoint for testing and validation without impacting the primary request flow.

## How It Works

When a request is processed, the plugin asynchronously sends a copy to a configured shadow endpoint. The shadow response is discarded and doesn't affect the original request/response cycle.

```
Request ──► Gateway ──► Upstream (primary response)
              │
              └──► Shadow Endpoint (async, discarded)
```

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `target` | String | ✅ | - | URL of shadow endpoint |
| `percentage` | Integer | ❌ | 100 | Percentage of traffic to mirror (1-100) |
| `timeout` | Integer | ❌ | 5000 | Shadow request timeout in ms |

## Example Configuration

### Basic Configuration

```yaml
plugins:
  - name: shadow-traffic
    config:
      target: http://shadow-service:8080
```

### Percentage-Based Sampling

```yaml
plugins:
  - name: shadow-traffic
    config:
      target: http://canary-service:8080
      percentage: 10  # Mirror 10% of traffic
      timeout: 3000
```

## Use Cases

1. **Canary Validation**: Test new service versions with production traffic
2. **Performance Testing**: Compare latency between old and new implementations
3. **A/B Testing Backend**: Validate changes before full rollout
4. **Disaster Recovery**: Warm standby systems

## Best Practices

::: tip
Use shadow traffic to validate new deployments without user impact.
:::

::: warning
Shadow traffic doubles load on your network. Adjust `percentage` accordingly.
:::

## Related Plugins

- [Cache Plugin](./cache.md) - Note: cached responses won't trigger shadow requests
- [Rate Limit Plugin](./rate-limit.md) - Shadow requests bypass rate limiting

For more plugins, visit the **[Plugins Overview](./index.md)**.
