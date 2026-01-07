# Multi-Auth Plugin

The Multi-Auth (`multi-auth`) plugin allows multiple authentication methods to be accepted for a single route, enabling flexible authentication strategies.

## How It Works

The plugin attempts authentication using multiple configured methods in order. If any method succeeds, the request is allowed. This enables scenarios like supporting both JWT tokens and API keys for the same endpoint.

## Configuration Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `methods` | Array | ✅ | List of auth method names to try |
| `any_pass` | Boolean | ❌ | Allow if any method passes (default: true) |

## Example Configuration

### JWT or API Key

```yaml
plugins:
  - name: multi-auth
    config:
      methods:
        - jwt
        - key-auth
```

### All Methods Must Pass

```yaml
plugins:
  - name: multi-auth
    config:
      methods:
        - jwt
        - mtls
      any_pass: false  # Both JWT and mTLS required
```

## Authentication Order

Methods are evaluated in the order specified. On first successful authentication, the request proceeds.

```
Request ──► Try JWT ──► Success? ──► Proceed
                │
                └── Fail ──► Try Key-Auth ──► Success? ──► Proceed
                                    │
                                    └── Fail ──► 401 Unauthorized
```

## Use Cases

1. **Migration**: Support legacy API keys while transitioning to JWT
2. **Flexibility**: Allow both user tokens and service API keys
3. **Defense in Depth**: Require multiple authentication factors

## Response When All Fail

```json
{
  "error": "UNAUTHORIZED",
  "message": "All authentication methods failed"
}
```

HTTP Status: `401 Unauthorized`

## Related Plugins

- [JWT Plugin](./jwt.md) - JSON Web Token authentication
- [Key Auth Plugin](./key-auth.md) - API key authentication
- [Basic Auth Plugin](./basic-auth.md) - Basic HTTP authentication
- [mTLS Plugin](./mtls.md) - Client certificate authentication

For more plugins, visit the **[Plugins Overview](./index.md)**.
