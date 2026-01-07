# Bot Protection Plugin

Production-grade bot and crawler protection with **80+ known malicious bot patterns** and intelligent browser detection.

## Protection

- **Vulnerability Scanners**: SQLMap, Nikto, Burp Suite, Acunetix, etc.
- **Scrapers**: Scrapy, Python requests, Puppeteer, Selenium
- **Bad Bots**: SEO crawlers, backlink checkers, aggressive scrapers
- **DDoS Tools**: Slowloris, Vegeta, k6, JMeter, Locust

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | String | "block" | "block" (blacklist) or "allow" (whitelist only) |
| `blacklist` | Array | [] | Additional patterns to block |
| `whitelist` | Array | [] | Patterns to always allow |
| `use_default_blacklist` | Boolean | true | Use built-in 80+ malicious bot list |
| `allow_good_bots` | Boolean | true | Allow search engines & social bots |
| `block_empty_ua` | Boolean | false | Block empty User-Agent |
| `validate_browser_headers` | Boolean | false | Validate Accept/Accept-Language headers |
| `block_message` | String | "Access denied" | Custom error message |

## Example Configuration

### Production (Recommended)

```yaml
plugins:
  - name: bot-protection
    config:
      use_default_blacklist: true
      allow_good_bots: true
```

### High Security (API)

```yaml
plugins:
  - name: bot-protection
    config:
      use_default_blacklist: true
      block_empty_ua: true
      validate_browser_headers: true
```

### Custom Blacklist

```yaml
plugins:
  - name: bot-protection
    config:
      blacklist:
        - "my-bad-bot"
        - "custom-scraper"
      whitelist:
        - "my-monitoring-tool"
```

### Whitelist Mode (Strict)

```yaml
plugins:
  - name: bot-protection
    config:
      mode: "allow"
      whitelist:
        - "googlebot"
        - "bingbot"
        - "my-api-client"
```

## Built-in Bot Lists

### Blocked by Default (80+ patterns)

| Category | Examples |
|----------|----------|
| Scanners | sqlmap, nikto, burpsuite, acunetix, nmap |
| Scrapers | scrapy, python-requests, puppeteer, selenium |
| Bad SEO | ahrefsbot, semrushbot, mj12bot, dotbot |
| DDoS | slowloris, vegeta, k6, locust, jmeter |

### Allowed Good Bots

| Category | Examples |
|----------|----------|
| Search | googlebot, bingbot, duckduckbot, yandexbot |
| Social | facebookexternalhit, twitterbot, linkedinbot |
| Monitors | uptimerobot, pingdom, statuscake |

## Response

```json
{
  "error": "BOT_DETECTED",
  "message": "Access denied"
}
```

HTTP Status: `403 Forbidden`

## Logging

```
Bot protection: Request blocked
  reason=blacklisted_bot
  user_agent=sqlmap/1.5
  client_ip=192.168.1.50
  path=/api/users
```

## Use with WAF

For maximum protection, combine with WAF:

```yaml
plugins:
  - name: waf
    config:
      paranoia_level: 1

  - name: bot-protection
    config:
      use_default_blacklist: true
```

## Related

- [WAF Plugin](./waf.md) - Attack detection
- [Rate Limit Plugin](./rate-limit.md) - Brute force protection
- [ACL Plugin](./acl.md) - IP-based access control
