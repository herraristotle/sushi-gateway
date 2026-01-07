# Sanitization Plugin

The Sanitization (`sanitization`) plugin cleans and validates request input to prevent injection attacks and ensure data integrity.

## How It Works

The plugin inspects request inputs (query parameters, headers, body) and removes or escapes potentially dangerous characters before forwarding to upstream services.

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `strip_html` | Boolean | ❌ | true | Remove HTML tags |
| `strip_scripts` | Boolean | ❌ | true | Remove script tags |
| `encode_special` | Boolean | ❌ | true | Encode special characters |

## Example Configuration

```yaml
plugins:
  - name: sanitization
    config:
      strip_html: true
      strip_scripts: true
      encode_special: true
```

## Use Cases

1. **Input Cleaning**: Remove potentially dangerous content
2. **XSS Prevention**: Strip client-side script injection
3. **Data Normalization**: Ensure consistent input format

## Related Plugins

- [WAF Plugin](./waf.md) - Block malicious requests
- [Request Size Limit Plugin](./request-size-limit.md) - Limit payload size

For more plugins, visit the **[Plugins Overview](./index.md)**.
