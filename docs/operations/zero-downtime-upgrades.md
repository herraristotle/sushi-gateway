# Zero-Downtime Upgrade Guide

This runbook covers upgrading Sushi Gateway without service interruption.

## Prerequisites

- Multiple gateway replicas running
- Health checks configured
- Rolling deployment capability (Kubernetes/Docker Swarm)

## Upgrade Procedure

### Step 1: Verify Current State

```bash
# Check all replicas are healthy
kubectl get pods -l app=sushi-proxy

# Verify health endpoints
curl http://localhost:8081/api/health
```

### Step 2: Prepare New Configuration

```bash
# Validate new configuration
./sushi-proxy --validate-config --config new-config.yaml

# Create ConfigMap with new config
kubectl create configmap sushi-config-new --from-file=config.yaml=new-config.yaml
```

### Step 3: Rolling Update

::: warning
Ensure `maxUnavailable: 0` to prevent service interruption.
:::

```yaml
# deployment.yaml
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
```

```bash
# Apply new image/config
kubectl set image deployment/sushi-proxy sushi-proxy=rawsashimi/sushi-proxy:v2.0.0

# Watch rollout
kubectl rollout status deployment/sushi-proxy
```

### Step 4: Verify Upgrade

```bash
# Check new pods are healthy
kubectl get pods -l app=sushi-proxy

# Verify version
curl http://localhost:8081/api/gateway | jq '.version'

# Check metrics for errors
curl http://localhost:8081/metrics | grep error
```

### Step 5: Rollback (If Needed)

```bash
# Immediate rollback
kubectl rollout undo deployment/sushi-proxy

# Rollback to specific revision
kubectl rollout undo deployment/sushi-proxy --to-revision=2
```

## Docker Compose Upgrade

```bash
# Pull new image
docker compose pull sushi-proxy

# Rolling restart
docker compose up -d --no-deps sushi-proxy

# Verify
docker compose ps
```

## Checklist

- [ ] Backup current configuration
- [ ] Validate new configuration
- [ ] Verify health checks pass
- [ ] Monitor error rates during rollout
- [ ] Confirm new version is running
- [ ] Update documentation

## Related

- [Deployment Guide](../getting-started/deployment.md)
- [Troubleshooting](../troubleshooting/common-issues.md)
