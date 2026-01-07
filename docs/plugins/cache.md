# Cache Plugin

The Cache (`cache`) plugin provides response caching using Redis as a backend store. It reduces upstream load and improves response times by serving cached responses for repeated requests.

## How It Works

The Cache plugin intercepts GET requests and checks Redis for a cached response. On a cache hit, it returns the cached response immediately. On a cache miss, it forwards the request to the upstream and caches successful responses.

### Key Features

- Redis-backed distributed caching
- Configurable TTL (time-to-live)
- Cache-Control header respect
- X-Cache response headers for debugging

::: tip
Requires Redis connection. Configure `REDIS_ADDR` environment variable.
:::

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `ttl` | Integer | ✅ | 60 | Cache TTL in seconds |
| `cache_control` | Boolean | ❌ | true | Respect Cache-Control headers |

## Example Configuration

### Minimal Configuration

```yaml
plugins:
  - name: cache
    config:
      ttl: 300
```

### Advanced Configuration

```yaml
plugins:
  - name: cache
    config:
      ttl: 3600           # 1 hour cache
      cache_control: true # Respect no-cache, no-store, private
```

## Response Headers

The plugin adds informational headers to responses:

| Header | Values | Description |
|--------|--------|-------------|
| `X-Cache` | `HIT` / `MISS` | Whether response was served from cache |
| `X-Cache-Age` | `<seconds>` | Age of cached response in seconds |

## Cache Key Generation

Cache keys are generated from:
- HTTP Method
- Host
- Full request URI (path + query string)

Example: `sushi:cache:GET:api.example.com/v1/users?page=1`

## Cacheable Responses

Only responses meeting these criteria are cached:
- HTTP status code 200-299
- GET requests only
- Not containing `Cache-Control: no-store`, `no-cache`, or `private` (when `cache_control: true`)

## Use Cases

1. **API Response Caching**: Cache frequently requested, rarely changing data
2. **Static Content**: Cache configuration or lookup data
3. **Rate Limit Bypass**: Reduce upstream load for popular endpoints

## Performance Considerations

- Redis latency adds ~1-2ms per request
- Consider TTL based on data freshness requirements
- Monitor cache hit ratio via `/metrics`

::: warning
Cache invalidation is time-based only. For immediate invalidation, reduce TTL or restart the gateway.
:::

## Related Plugins

- [Rate Limiting Plugin](./rate-limit.md) - Often used together
- [HTTP Log Plugin](./http-log.md) - Log cache hits/misses

For more plugins, visit the **[Plugins Overview](./index.md)**.
