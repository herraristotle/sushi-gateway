# WAF Plugin

Production-grade Web Application Firewall using **[Coraza](https://coraza.io/)** with embedded **OWASP Core Rule Set** (1000+ rules).

## Protection

- SQL Injection (SQLi)
- Cross-Site Scripting (XSS)
- Path Traversal / Local File Inclusion
- Remote Code Execution
- XML External Entity (XXE)
- Server-Side Request Forgery (SSRF)
- Protocol Attacks
- Scanner/Bot Detection

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `paranoia_level` | Integer (1-4) | 1 | OWASP CRS sensitivity level |
| `observation_mode` | Boolean | false | Log attacks without blocking |
| `rules_file` | String | "" | Additional custom rules |
| `rules_dir` | String | "" | Directory with .conf files |

### Paranoia Levels

| Level | Protection | False Positives |
|-------|------------|-----------------|
| **1** | Basic, common attacks | Very low |
| **2** | Moderate | Low |
| **3** | High | Moderate |
| **4** | Maximum | May require tuning |

## Example Configuration

### Production (Recommended)

```yaml
plugins:
  - name: waf
    config:
      paranoia_level: 1
```

### High Security

```yaml
plugins:
  - name: waf
    config:
      paranoia_level: 3
```

### Testing/Staging (Observation Mode)

```yaml
plugins:
  - name: waf
    config:
      paranoia_level: 2
      observation_mode: true  # Log only, no blocking
```

### With Custom Rules

```yaml
plugins:
  - name: waf
    config:
      paranoia_level: 1
      rules_file: /etc/sushi-gateway/custom-waf.conf
```

## Security Behavior

| Scenario | Behavior |
|----------|----------|
| Attack detected | **403 Forbidden** |
| WAF initialization fails | **503 Service Unavailable** (fail-closed) |
| Observation mode | Logged only, request continues |

## Response

```json
{
  "error": "WAF_BLOCKED",
  "message": "Request blocked by security policy [942100]"
}
```

## Custom Rules (SecLang)

```apache
# Whitelist specific IP
SecRule REMOTE_ADDR "@eq 10.0.0.100" "id:100000,phase:1,allow,nolog"

# Block specific pattern
SecRule ARGS "@contains malicious" "id:100001,phase:2,deny,status:403,msg:'Custom rule'"

# Rate limiting via anomaly scoring
SecRule TX:ANOMALY_SCORE "@ge 10" "id:100002,phase:5,deny,status:429"
```

## Logging

```
WAF: Attack blocked
  rule_id=942100
  action=deny
  phase=phase2
  client_ip=192.168.1.50
  method=POST
  path=/api/users
  user_agent=curl/7.68.0
```

## Related

- [OWASP CRS](https://coreruleset.org/)
- [Coraza Documentation](https://coraza.io/docs/)
- [ACL Plugin](./acl.md) - IP-based access control
- [Rate Limit Plugin](./rate-limit.md) - Brute force protection
