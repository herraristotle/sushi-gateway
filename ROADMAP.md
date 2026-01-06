# Project Roadmap

## Short-term Goals (1-3 months)

### Documentation

- [x] Create roadmap
- [x] Create docker compose guide
- [x] Developer getting started guide (VitePress docs)

### Infrastructure & Deployment

- [ ] Create Helm charts for Kubernetes deployment
  - [ ] Define configurable values and templates
  - [ ] Document installation and configuration steps

### Observability

- [ ] Implement OpenTelemetry integration
- [x] Add Prometheus metrics

### Circuit Breaker

- [x] Implement circuit breaker pattern

### Load Testing

- [ ] Implement load testing with k6

## Medium-term Goals (3-6 months)

### Proper asset designs

- [ ] Create a proper asset design for the project (logo, favicon, etc.)
- [ ] Enhance landing page

### Load Balancing

- [x] Implement weighted load balancing
  - [x] Add configuration options for weight distribution
  - [x] Support dynamic weight adjustments
- [x] Implement Least Connections algorithm
- [x] Implement Consistent Hashing
- [x] Implement Latency-based Routing

### Plugin System

- [ ] Develop new plugin types
  - [ ] Authentication plugins
  - [ ] Custom middleware plugins
  - [ ] Data transformation plugins

## Long-term Goals (6+ months)

### UI Modernization

- [x] Design new UI architecture
- [x] Upstreams page with real-time metrics
- [x] Health dashboard

### Protocol Support

- [ ] WebSocket proxying
- [ ] gRPC proxying
- [ ] GraphQL support

### Service Discovery

- [ ] Consul integration
- [ ] Kubernetes service discovery
- [ ] DNS-based discovery

### Advanced Features

- [ ] A/B testing support
- [ ] Traffic shadowing
- [ ] Request/response transformations
- [ ] Custom Lua scripting

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

Priorities for contributors:
1. Documentation improvements
2. New plugins
3. Performance optimizations
4. Bug fixes
