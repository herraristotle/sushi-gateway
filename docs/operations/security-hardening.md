# Security Hardening Guide

This runbook covers security best practices and hardening procedures for Sushi Gateway.

## Security Checklist

### Authentication & Authorization
- [ ] Enable authentication on all routes (JWT, Key-Auth, or mTLS)
- [ ] Configure RBAC for fine-grained access control
- [ ] Rotate secrets and API keys regularly
- [ ] Use short-lived JWT tokens

### Network Security
- [ ] Enable TLS for all external traffic
- [ ] Restrict Admin API access to internal network
- [ ] Configure firewall rules
- [ ] Use network policies in Kubernetes

### Plugin Security
- [ ] Enable rate limiting to prevent abuse
- [ ] Enable WAF for SQL injection/XSS protection
- [ ] Configure ACL for IP filtering
- [ ] Enable bot protection

## TLS Configuration

### Enable HTTPS

```yaml
services:
  - name: secure-api
    protocol: https
    tls:
      enabled: true
      cert_file: /certs/tls.crt
      key_file: /certs/tls.key
```

### mTLS for Upstreams

```yaml
plugins:
  - name: mtls
    config:
      ca_certificates: /certs/ca.crt
      client_certificate: /certs/client.crt
      client_key: /certs/client.key
```

## Admin API Security

::: warning
Never expose the Admin API (port 8081) to the public internet!
:::

### Kubernetes Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: admin-api-policy
spec:
  podSelector:
    matchLabels:
      app: sushi-proxy
  ingress:
    - from:
        - podSelector:
            matchLabels:
              role: admin
      ports:
        - port: 8081
```

### Environment Variables

```bash
# Required for Admin API access
ADMIN_USER=secure-admin
ADMIN_PASSWORD=<strong-password>
```

## Rate Limiting

Protect against abuse:

```yaml
plugins:
  - name: rate-limiting
    config:
      second: 100
      minute: 1000
      hour: 10000
```

## WAF Protection

```yaml
plugins:
  - name: waf
    config:
      sql_injection: true
      xss: true
```

## Secret Management

### Kubernetes Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: gateway-secrets
type: Opaque
stringData:
  jwt-secret: "<your-jwt-secret>"
  admin-password: "<admin-password>"
```

### External Secret Stores

For production, use:
- HashiCorp Vault
- AWS Secrets Manager
- Azure Key Vault
- GCP Secret Manager

## Audit Logging

Enable HTTP logging for audit trail:

```yaml
plugins:
  - name: http-log
    config:
      http_endpoint: https://audit-service/logs
      method: POST
      content_type: application/json
```

## Security Headers

Add security headers via header transformation:

```yaml
plugins:
  - name: header-transformation
    config:
      add:
        X-Content-Type-Options: nosniff
        X-Frame-Options: DENY
        X-XSS-Protection: "1; mode=block"
        Strict-Transport-Security: "max-age=31536000; includeSubDomains"
```

## Vulnerability Scanning

Regularly scan gateway container images:

```bash
# Using Trivy
trivy image rawsashimi/sushi-proxy:latest

# Using Grype
grype rawsashimi/sushi-proxy:latest
```

## Incident Response

If security incident detected:

1. **Isolate** - Block suspicious IPs via ACL
2. **Investigate** - Check logs for attack patterns
3. **Remediate** - Patch vulnerabilities, rotate secrets
4. **Document** - Record incident details

```yaml
# Emergency IP block
plugins:
  - name: acl
    config:
      blacklist:
        - 192.168.1.100
        - 10.0.0.0/8
```

## Related

- [ACL Plugin](../plugins/acl.md)
- [WAF Plugin](../plugins/waf.md)
- [mTLS Plugin](../plugins/mtls.md)
- [RBAC Plugin](../plugins/rbac.md)
