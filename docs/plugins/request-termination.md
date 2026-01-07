# Request Termination Plugin

The Request Termination (`request-termination`) plugin immediately terminates requests with a configurable response, useful for maintenance mode, blocked endpoints, or deprecated APIs.

## How It Works

When a request matches a route with this plugin enabled, the gateway immediately returns the configured response without forwarding to any upstream service.

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `status_code` | Integer | ❌ | 503 | HTTP status code to return |
| `message` | String | ❌ | "Service Unavailable" | Response message |
| `body` | String | ❌ | - | Custom response body |
| `content_type` | String | ❌ | "application/json" | Response content type |

## Example Configuration

### Maintenance Mode

```yaml
routes:
  - name: maintenance
    paths: [/api/*]
    plugins:
      - name: request-termination
        config:
          status_code: 503
          message: "Service under maintenance. Please try again later."
```

### Deprecated Endpoint

```yaml
routes:
  - name: deprecated-api
    paths: [/api/v1/legacy]
    plugins:
      - name: request-termination
        config:
          status_code: 410
          message: "This API version has been deprecated. Please use /api/v2/"
```

### Block Endpoint

```yaml
routes:
  - name: blocked
    paths: [/admin/debug]
    plugins:
      - name: request-termination
        config:
          status_code: 403
          message: "Access forbidden"
```

## Response Format

Default JSON response:

```json
{
  "error": "SERVICE_UNAVAILABLE",
  "message": "Service under maintenance"
}
```

## Use Cases

1. **Maintenance Mode**: Return 503 during deployments
2. **Deprecated APIs**: Return 410 Gone with migration instructions
3. **Security**: Block sensitive endpoints
4. **Feature Flags**: Disable features without code changes

## Best Practices

::: tip
Apply at route level for specific endpoints, or service level for entire API maintenance.
:::

## Related Plugins

- [ACL Plugin](./acl.md) - IP-based access control
- [RBAC Plugin](./rbac.md) - Role-based access control

For more plugins, visit the **[Plugins Overview](./index.md)**.
