# Plugins in Sushi Gateway

Plugins are modular extensions that enhance the gateway's functionality. They can be used for tasks such as authentication, rate limiting, transformations, and more. Each plugin operates within a middleware chain, allowing precise control over how requests and responses are processed.

## What are Plugins?

Plugins are:

- Reusable components that add features to services and routes.
- Configurable to meet specific API requirements.
- Applied at different scopes (global, service, route) for fine-grained control.

::: tip
Learn about plugin fields and configurations in the **[Plugin Entity Documentation](../concepts/entities/plugin.md)**.
:::

## Plugin Middleware Chain

Plugins in Sushi Gateway operate in a defined middleware chain:

1. **Global Plugins**: Applied to all services and routes.
2. **Service-Level Plugins**: Applied to all routes within a specific service.
3. **Route-Level Plugins**: Applied to individual routes, overriding service and global plugins if applicable.

### Plugin Priority and Phases

The table below illustrates the priority and phases of specific plugins in Sushi Gateway. Plugins with higher priority values are executed earlier in the middleware chain.

| Priority | Phase    | Plugin                                     |
| -------- | -------- | ------------------------------------------ |
| 10000    | Response | Response Handler (logs request metadata)   |
| 3000     | Access   | Rate Limit (High performance Leaky Bucket) |
| 2500     | Access   | Bot Protection                             |
| 2000     | Access   | Cross Origin Resource Sharing (RFC 6454)   |
| 1600     | Access   | Mutual Transport Layer Security (RFC 8705) |
| 1450     | Access   | JSON Web Token (RFC 7519)                  |
| 1250     | Access   | API Key Authentication                     |
| 1100     | Access   | Basic Authentication (RFC 7617)            |
| 1000     | Access   | Web Application Firewall (WAF)             |
| 951      | Access   | Request Size Limit                         |
| 950      | Access   | Access Control List                        |
| 910      | Access   | Circuit Breaker                            |
| 905      | Access   | RBAC (Role-Based Access Control)           |
| 500      | Access   | Multi-Auth (Auth-OR logic)                 |
| 450      | Access   | Header Transformation                      |
| 100      | Access   | Shadow Traffic (Mirroring)                 |
| 12       | Log      | HTTP Log                                   |

::: tip
Use route-level plugins for the highest level of specificity and ensure priority alignment with your gateway logic.
:::

#### Plugin Phases

Plugins are executed in separate phases to ensure certain tasks (like logging) always run even if a request is blocked.

::: info
Phases occur in the following order:

1. **Access Phase**: Authentication, security, and traffic control.
2. **Response Phase**: Metadata recording and header injection.
3. **Log Phase**: Global observability and audit logs.
:::

## Available Plugins

Sushi Gateway supports **21 plugins** organized into categories.

### Authentication Plugins

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `basic-auth` | Basic HTTP authentication (RFC 7617). | [Basic Auth Plugin](./basic-auth.md) |
| `jwt` | JSON Web Token validation (RFC 7519). | [JWT Plugin](./jwt.md) |
| `key-auth` | API Key authentication. | [Key Auth Plugin](./key-auth.md) |
| `mtls` | Mutual TLS client certificates. | [mTLS Plugin](./mtls.md) |
| `multi-auth` | Multiple authentication methods (OR logic). | [Multi-Auth Plugin](./multi-auth.md) |

### Security Plugins

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `acl` | IP whitelist/blacklist access control. | [ACL Plugin](./acl.md) |
| `bot-protection` | Block automated bots (80+ patterns). | [Bot Protection Plugin](./bot-protection.md) |
| `cors` | Cross-Origin Resource Sharing (RFC 6454). | [CORS Plugin](./cors.md) |
| `rbac` | Role-Based Access Control (Casbin). | [RBAC Plugin](./rbac.md) |
| `waf` | Coraza Web Application Firewall + CRS. | [WAF Plugin](./waf.md) |
| `sanitization` | Input sanitization policies. | [Sanitization Plugin](./sanitization.md) |

### Traffic Control & Resilience

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `rate-limiting` | Distributed rate limiting (Redis). | [Rate Limit Plugin](./rate-limit.md) |
| `request-size-limiting` | Limit request body size. | [Request Size Limit Plugin](./request-size-limit.md) |
| `request-termination` | Block/terminate requests. | [Request Termination Plugin](./request-termination.md) |
| `circuit-breaker` | Fault tolerance for upstream services. | [Circuit Breaker Plugin](./circuit-breaker.md) |

### Transformation & Testing

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `header-transformation` | Modify request/response headers. | [Header Transformation](./header-transformation.md) |
| `shadow-traffic` | Traffic mirroring / shadow testing. | [Shadow Traffic](./shadow-traffic.md) |

### Performance & Caching

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `cache` | Response caching with Redis. | [Cache Plugin](./cache.md) |

### Observability

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `http-log` | HTTP endpoint logging. | [HTTP Log Plugin](./http-log.md) |
| `prometheus` | Prometheus metrics and instrumentation. | [Prometheus Plugin](./prometheus.md) |
| `opentelemetry` | Distributed tracing (OTLP). | [OpenTelemetry](./opentelemetry.md) |

::: tip
Click on a plugin name to learn more about its configuration and use cases.
:::

## Example Plugin Configuration

Here’s how to configure a `rate_limit` plugin:

```json
{
  "name": "rate_limit",
  "enabled": true,
  "config": {
    "limit_second": 10,
    "limit_min": 100,
    "limit_hour": 1000
  }
}
```

### Explanation

- **`name`**: The plugin type (e.g., `rate_limit`).
- **`enabled`**: Toggles the plugin on or off.
- **`config`**: Plugin-specific settings.

## Tips for Using Plugins

::: tip
Combine multiple plugins at the route level to customize behavior for specific APIs.
:::
