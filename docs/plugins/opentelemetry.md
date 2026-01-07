# OpenTelemetry Plugin

The OpenTelemetry (`opentelemetry`) plugin provides distributed tracing by exporting spans to an OpenTelemetry Collector.

## How It Works

When enabled, the plugin creates spans for each request and exports them to the configured collector. Spans include timing, status, and custom attributes for request analysis.

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `endpoint` | String | ✅ | - | OTLP collector endpoint (gRPC) |
| `resource_attributes` | Object | ❌ | - | Static resource attributes |
| `headers` | Object | ❌ | - | Headers for collector auth |
| `batch_span_count` | Integer | ❌ | 200 | Spans per batch |
| `batch_flush_delay` | Integer | ❌ | 3 | Seconds between flushes |

## Example Configuration

### Basic Configuration

```yaml
plugins:
  - name: opentelemetry
    config:
      endpoint: http://otel-collector:4317
```

### Production Configuration

```yaml
plugins:
  - name: opentelemetry
    config:
      endpoint: http://otel-collector:4317
      resource_attributes:
        service.name: sushi-gateway
        service.version: "1.0.0"
        deployment.environment: production
      headers:
        X-Auth-Token: "collector-secret"
      batch_span_count: 200
      batch_flush_delay: 3
```

## Span Attributes

Each span includes:

| Attribute | Description |
|-----------|-------------|
| `http.method` | HTTP method (GET, POST, etc.) |
| `http.url` | Full request URL |
| `http.status_code` | Response status code |
| `http.route` | Matched route pattern |
| `service.name` | Target service name |

## Collector Setup

Example `otel-collector-config.yaml`:

```yaml
receivers:
  otlp:
    protocols:
      grpc:

exporters:
  jaeger:
    endpoint: jaeger:14250
  
  logging:
    loglevel: debug

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [jaeger, logging]
```

## Use Cases

1. **Request Tracing**: Follow requests across services
2. **Performance Analysis**: Identify slow operations
3. **Debugging**: Trace failed requests through the system
4. **Service Mapping**: Visualize service dependencies

## Visualization

Export traces to:
- **Jaeger**: Open-source distributed tracing
- **Zipkin**: Alternative tracing backend
- **Grafana Tempo**: Scalable tracing storage
- **Cloud Providers**: AWS X-Ray, Google Cloud Trace

## Best Practices

::: tip
Use sampling in high-traffic environments to reduce overhead:
- 100% for development
- 10-50% for production
:::

## Related Plugins

- [Prometheus Plugin](./prometheus.md) - Metrics collection
- [HTTP Log Plugin](./http-log.md) - Request logging

For more plugins, visit the **[Plugins Overview](./index.md)**.
