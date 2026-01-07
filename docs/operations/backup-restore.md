# Backup and Disaster Recovery

This runbook covers backup procedures and disaster recovery for Sushi Gateway.

## What to Backup

| Component | Type | Frequency |
|-----------|------|-----------|
| Configuration files | YAML/JSON | On every change |
| Kubernetes manifests | YAML | On every change |
| RBAC policies | CSV | On every change |
| TLS certificates | PEM | Monthly |

::: tip
Sushi Gateway is **stateless** (db-less mode). All state is in configuration files.
:::

## Backup Procedures

### Configuration Backup

```bash
# Kubernetes ConfigMap backup
kubectl get configmap sushi-config -o yaml > backup/sushi-config-$(date +%Y%m%d).yaml

# Direct file backup
cp /path/to/config.yaml backup/config-$(date +%Y%m%d).yaml
```

### Automated Backup Script

```bash
#!/bin/bash
# backup-gateway.sh

BACKUP_DIR="/backups/sushi-gateway"
DATE=$(date +%Y%m%d-%H%M%S)

mkdir -p $BACKUP_DIR

# Backup ConfigMap
kubectl get configmap sushi-config -o yaml > $BACKUP_DIR/config-$DATE.yaml

# Backup secrets
kubectl get secret sushi-secrets -o yaml > $BACKUP_DIR/secrets-$DATE.yaml

# Keep last 30 days
find $BACKUP_DIR -mtime +30 -delete

echo "Backup completed: $DATE"
```

### Git-Based Backup

::: info
Recommended: Store configuration in Git for version control.
:::

```bash
# Initialize git repo for configs
cd /path/to/gateway-config
git init
git add .
git commit -m "Initial configuration"
git remote add origin git@github.com:org/gateway-config.git
git push -u origin main
```

## Disaster Recovery

### Scenario 1: Configuration Corruption

```bash
# Restore from backup
kubectl apply -f backup/sushi-config-20260106.yaml

# Restart gateway
kubectl rollout restart deployment/sushi-proxy
```

### Scenario 2: Complete Cluster Failure

```bash
# 1. Set up new cluster
# 2. Apply manifests
kubectl apply -f backup/namespace.yaml
kubectl apply -f backup/sushi-config.yaml
kubectl apply -f backup/sushi-secrets.yaml
kubectl apply -f backup/sushi-deployment.yaml
kubectl apply -f backup/sushi-service.yaml

# 3. Verify
kubectl get pods -l app=sushi-proxy
```

### Scenario 3: Rollback to Previous Version

```bash
# Kubernetes rollback
kubectl rollout undo deployment/sushi-proxy

# Or restore specific config version
kubectl apply -f backup/sushi-config-20260105.yaml
kubectl rollout restart deployment/sushi-proxy
```

## Recovery Time Objectives

| Scenario | RTO | RPO |
|----------|-----|-----|
| Config rollback | < 5 min | 0 (if Git-backed) |
| Pod failure | < 1 min | 0 |
| Node failure | < 5 min | 0 |
| Cluster failure | < 30 min | Last backup |

## Testing Recovery

### Monthly DR Test

1. Create test environment
2. Restore from backup
3. Verify all routes work
4. Document any issues

```bash
# Test restore in staging
kubectl config use-context staging
kubectl apply -f backup/sushi-config-latest.yaml
kubectl rollout restart deployment/sushi-proxy

# Verify
curl https://staging-gateway/health
```

## Checklist

- [ ] Automated daily backups configured
- [ ] Backups stored off-cluster
- [ ] Git repository for configuration
- [ ] DR procedure tested quarterly
- [ ] RTO/RPO documented and validated

## Related

- [Zero-Downtime Upgrades](./zero-downtime-upgrades.md)
- [Incident Response](./incident-response.md)
