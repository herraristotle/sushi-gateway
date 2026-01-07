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
| 2500     | Access   | Bot Protection                             |
| 2000     | Access   | Cross Origin Resource Sharing (RFC 6454)   |
| 1600     | Access   | Mutual Transport Layer Security (RFC 8705) |
| 1450     | Access   | JSON Web Token (RFC 7519)                  |
| 1250     | Access   | API Key Authentication                     |
| 1100     | Access   | Basic Authentication (RFC 7617)            |
| 951      | Access   | Request Size Limit                         |
| 950      | Access   | Access Control List                        |
| 910      | Access   | Rate Limit                                 |
| 12       | Log      | HTTP Log                                   |

::: tip
Use route-level plugins for the highest level of specificity and ensure priority alignment with your gateway logic.
:::

#### Plugin Phases

Plugins are executed in seperate phases, this is to ensure that certain plugins have guaranteed execution - like logging regardless of whether the request was successful or not.

::: info
Phases occur in the following order:

1. Access Phase
2. Response Phase
3. Log Phase

:::

- **Access Phase**: Plugins that are executed during the access phase handle authentication, authorization, and other security-related tasks.
- **Response Phase**: Plugins that are executed during the response phase handle response processing tasks like recording metadata.
- **Log Phase**: Plugins that are executed during the log phase handle logging and monitoring tasks.

## Available Plugins

Sushi Gateway supports **21 plugins** organized into categories. The table below provides a complete overview:

### Authentication Plugins

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `basic-auth` | Basic HTTP authentication (RFC 7617). | [Basic Auth Plugin](./basic-auth.md) |
| `jwt` | JSON Web Token validation (RFC 7519). | [JWT Plugin](./jwt.md) |
| `key-auth` | API Key authentication. | [Key Auth Plugin](./key-auth.md) |
| `mtls` | Mutual TLS client certificates. | [mTLS Plugin](./mtls.md) |
| `multi-auth` | Multiple authentication methods. | [Multi-Auth Plugin](./multi-auth.md) |

### Security Plugins

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `acl` | IP whitelist/blacklist access control. | [ACL Plugin](./acl.md) |
| `bot-protection` | Block automated bots. | [Bot Protection Plugin](./bot-protection.md) |
| `cors` | Cross-Origin Resource Sharing (RFC 6454). | [CORS Plugin](./cors.md) |
| `rbac` | Role-Based Access Control (Casbin). | [RBAC Plugin](./rbac.md) |
| `waf` | Web Application Firewall. | [WAF Plugin](./waf.md) |
| `sanitization` | Input sanitization. | [Sanitization Plugin](./sanitization.md) |

### Traffic Control Plugins

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `rate-limiting` | Distributed rate limiting. | [Rate Limit Plugin](./rate-limit.md) |
| `request-size-limiting` | Limit request body size. | [Request Size Limit Plugin](./request-size-limit.md) |
| `request-termination` | Block/terminate requests. | [Request Termination Plugin](./request-termination.md) |
| `circuit-breaker` | Fault tolerance via gobreaker. | [Circuit Breaker Plugin](./circuit-breaker.md) |

### Transformation Plugins

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `header-transformation` | Modify request/response headers. | [Header Transformation Plugin](./header-transformation.md) |
| `request-transformer` | Kong-compatible alias for header-transformation. | [Header Transformation Plugin](./header-transformation.md) |

### Performance Plugins

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `cache` | Response caching with Redis. | [Cache Plugin](./cache.md) |

### Observability Plugins

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `http-log` | HTTP endpoint logging. | [HTTP Log Plugin](./http-log.md) |
| `prometheus` | Prometheus metrics. | [Prometheus Plugin](./prometheus.md) |
| `opentelemetry` | Distributed tracing. | [OpenTelemetry Plugin](./opentelemetry.md) |

### Testing Plugins

| Plugin Name | Description | Documentation |
|-------------|-------------|---------------|
| `shadow-traffic` | Traffic mirroring. | [Shadow Traffic Plugin](./shadow-traffic.md) |

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
