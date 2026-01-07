# RBAC Plugin

The RBAC (`rbac`) plugin provides Role-Based Access Control using [Casbin](https://casbin.org/), a powerful authorization library. It enforces fine-grained access policies based on user roles, request paths, and HTTP methods.

## How It Works

The RBAC plugin extracts the subject (user/role) from JWT claims or Basic Auth context, then evaluates access against Casbin policies. Requests are allowed or denied based on policy matching.

### Authorization Flow

1. Extract subject from JWT `sub`/`role` claim or Basic Auth username
2. Get object (request path) and action (HTTP method)
3. Evaluate against Casbin policy
4. Allow or deny with 403 Forbidden

## Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `model_path` | String | ❌ | `config/rbac/model.conf` | Casbin model file path |
| `policy_path` | String | ❌ | `config/rbac/policy.csv` | Casbin policy file path |

## Example Configuration

### Basic Configuration

```yaml
plugins:
  - name: rbac
    config:
      model_path: config/rbac/model.conf
      policy_path: config/rbac/policy.csv
```

## Casbin Model File

Create `config/rbac/model.conf`:

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && keyMatch(r.obj, p.obj) && r.act == p.act
```

## Casbin Policy File

Create `config/rbac/policy.csv`:

```csv
p, admin, /*, *
p, user, /api/users/*, GET
p, user, /api/profile, *
p, guest, /api/public/*, GET
```

### Policy Explanation

| Subject | Object | Action | Meaning |
|---------|--------|--------|---------|
| `admin` | `/*` | `*` | Admin can access everything |
| `user` | `/api/users/*` | `GET` | Users can read user data |
| `user` | `/api/profile` | `*` | Users can manage their profile |
| `guest` | `/api/public/*` | `GET` | Guests can read public data |

## Subject Extraction

The plugin extracts subjects in this order:

1. **JWT Claims**: `sub`, `username`, or `role` claim
2. **Basic Auth**: Username from context
3. **Default**: `anonymous` if no auth context

## Response When Denied

```json
{
  "error": "ACCESS_DENIED",
  "message": "Access Denied"
}
```

HTTP Status: `403 Forbidden`

## Use Cases

1. **Role-Based APIs**: Different access levels for admin/user/guest
2. **Microservice Authorization**: Centralized access control
3. **Multi-Tenant APIs**: Tenant-specific resource access

## Example: Complete Setup

### 1. JWT with RBAC

```yaml
plugins:
  # First: Authenticate with JWT
  - name: jwt
    config:
      alg: HS256
      iss: my-issuer
      secret: my-secret
  
  # Then: Authorize with RBAC
  - name: rbac
    config:
      model_path: config/rbac/model.conf
      policy_path: config/rbac/policy.csv
```

### 2. JWT Payload

```json
{
  "sub": "admin",
  "exp": 1735689600
}
```

## Advanced Matchers

### Path Wildcards

```ini
# Match any path starting with /api/
m = keyMatch(r.obj, p.obj)

# Match path segments: /api/:id
m = keyMatch2(r.obj, p.obj)
```

### Role Groups

```csv
g, alice, admin
g, bob, user
p, admin, /admin/*, *
```

::: tip
Use `g` (grouping) policies to assign users to roles, then define permissions for roles.
:::

## Best Practices

- Place RBAC after authentication plugins (JWT, Basic Auth)
- Use wildcard patterns for scalable policies
- Keep policies in version control
- Test policies before deploying

## Related Plugins

- [JWT Plugin](./jwt.md) - Required for JWT-based subject extraction
- [Basic Auth Plugin](./basic-auth.md) - Alternative authentication
- [ACL Plugin](./acl.md) - IP-based access control

For more plugins, visit the **[Plugins Overview](./index.md)**.
