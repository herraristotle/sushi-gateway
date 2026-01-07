# Frequently Asked Questions

Common questions about Sushi Gateway.

## General

### What is Sushi Gateway?

Sushi Gateway is a lightweight, production-ready API Gateway built in Go. It provides enterprise-grade features comparable to Kong, KrakenD, and Tyk.

### What protocols does Sushi Gateway support?

- HTTP (port 8080)
- HTTPS (port 8443)
- Admin API (port 8081)

### Does Sushi Gateway require a database?

No. Sushi Gateway operates in **db-less mode** using declarative YAML/JSON configuration files. This simplifies deployment and enables GitOps workflows.

## Configuration

### How do I reload configuration without restart?

Currently, configuration changes require a gateway restart. Use rolling deployments in Kubernetes for zero-downtime updates.

### Can I use environment variables in configuration?

Yes. Use standard environment variable syntax in your configuration file:

```yaml
services:
  - name: api
    url: ${UPSTREAM_URL}
```

## Plugins

### How many plugins can I use?

There's no hard limit. However, each plugin adds processing overhead. Monitor latency impact when adding plugins.

### What's the plugin execution order?

Plugins execute by priority (lower number = earlier execution):
1. Bot Protection (2500)
2. CORS (2000)
3. mTLS (1600)
4. JWT (1450)
5. Key Auth (1250)
6. Basic Auth (1100)
7. ACL (950)
8. Rate Limit (910)

### Can I use multiple authentication plugins?

Yes. Use the `multi-auth` plugin to allow any of multiple authentication methods.

## Performance

### What throughput can Sushi Gateway handle?

Benchmarks show:
- 50,000+ requests/second on modern hardware
- Sub-millisecond added latency
- Linear scaling with replicas

### How do I improve latency?

1. Use latency-based load balancing
2. Enable response caching
3. Reduce plugin chain length
4. Add more replicas

## Troubleshooting

### Why are all my requests returning 503?

Check upstream health:
```bash
curl http://localhost:8081/api/health
```

Common causes: upstreams down, health check failures, circuit breaker open.

### Why am I getting rate limited?

Check rate limit stats:
```bash
curl http://localhost:8081/api/stats | jq '.rate_limits'
```

Adjust limits or ensure Redis is connected for distributed mode.

## Integration

### Does Sushi Gateway work with Kubernetes?

Yes. Deploy using:
- Kubernetes manifests
- Helm charts
- Docker Compose for development

### Can I use Sushi Gateway with service mesh?

Yes. Sushi Gateway works alongside service meshes like Istio or Linkerd as an ingress gateway.

## More Help

- [Troubleshooting Guide](../troubleshooting/common-issues.md)
- [GitHub Issues](https://github.com/rawsashimi1604/sushi-gateway/issues)
- [Discord Community](https://discord.gg/aPv4QhQ6)
