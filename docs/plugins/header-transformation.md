# Header Transformation Plugin

The Header Transformation (`header-transformation`) plugin modifies request headers before forwarding to upstream services. It supports adding, removing, renaming, replacing, and appending headers.

::: tip Kong Compatibility
This plugin is also available as `request-transformer` for Kong configuration compatibility.
:::

## How It Works

The plugin processes headers in a specific order to ensure predictable behavior:
1. **Remove** - Delete specified headers
2. **Rename** - Change header names
3. **Replace** - Update existing header values
4. **Append** - Add to existing header values
5. **Add** - Set new headers

## Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `add` | Object/Map | ❌ | Headers to add |
| `remove` | Array/Object | ❌ | Headers to remove |
| `rename` | Object/Map | ❌ | Headers to rename (old: new) |
| `replace` | Object/Map | ❌ | Headers to replace |
| `append` | Object/Map | ❌ | Values to append to headers |

::: info
At least one operation must be configured.
:::

## Example Configurations

### Add Headers

```yaml
plugins:
  - name: header-transformation
    config:
      add:
        X-Gateway-Version: "1.0"
        X-Request-Source: "api-gateway"
```

### Kong-Style Configuration

```yaml
plugins:
  - name: request-transformer
    config:
      add:
        headers:
          - "X-Gateway-Secret:my-secret"
          - "X-Forwarded-By:sushi-gateway"
```

### Remove Headers

```yaml
plugins:
  - name: header-transformation
    config:
      remove:
        - X-Debug-Info
        - X-Internal-Token
```

### Rename Headers

```yaml
plugins:
  - name: header-transformation
    config:
      rename:
        X-Old-Header: X-New-Header
        X-Legacy-Auth: Authorization
```

### Replace Header Values

```yaml
plugins:
  - name: header-transformation
    config:
      replace:
        User-Agent: SushiGateway/1.0
```

### Append to Headers

```yaml
plugins:
  - name: header-transformation
    config:
      append:
        X-Forwarded-For: ", gateway"
```

## Template Variables

The plugin supports basic template substitution for dynamic values:

```yaml
plugins:
  - name: header-transformation
    config:
      add:
        X-Consumer-ID: "$(headers['x-consumer-id'] or '')"
        X-Consumer-Username: "$(headers['x-consumer-username'] or '')"
        X-Authenticated-User: "$(headers['x-authenticated-userid'] or '')"
```

## Use Cases

1. **Identity Injection**: Add consumer identity headers for downstream services
2. **Header Normalization**: Rename legacy headers to standard names
3. **Security**: Remove sensitive headers before forwarding
4. **Debugging**: Add tracing/debugging headers

## Complete Example: Zero Trust Identity

```yaml
plugins:
  # First: Authenticate
  - name: jwt
    config:
      alg: HS256
      iss: my-issuer
      secret: my-secret
  
  # Then: Inject identity headers
  - name: header-transformation
    config:
      add:
        X-Kong-Consumer-ID: "$(headers['x-consumer-id'] or '')"
        X-Gateway-Secret: "shared-secret-123"
      remove:
        - Authorization  # Don't forward JWT to upstream
```

## Best Practices

::: tip
Use header transformation after authentication plugins to inject verified identity information.
:::

::: warning
Be careful when removing `Authorization` headers - ensure your upstream doesn't need them.
:::

## Related Plugins

- [JWT Plugin](./jwt.md) - Provides consumer identity context
- [Key Auth Plugin](./key-auth.md) - Alternative authentication
- [CORS Plugin](./cors.md) - Cross-origin header management

For more plugins, visit the **[Plugins Overview](./index.md)**.
