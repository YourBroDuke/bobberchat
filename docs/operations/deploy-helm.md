# Deploy with Helm

This guide covers deploying BobberChat using the Helm chart in `deploy/helm/bobberchat/`.

## Prerequisites

- Kubernetes cluster (1.24+)
- Helm 3.x installed
- Container image built and pushed to a registry
- Migration SQL file available locally (`migrations/001_initial_schema.sql`)

## Chart Overview

| Field | Value |
| --- | --- |
| Chart name | bobberchat |
| Chart version | 0.1.0 |
| App version | 1.0.0 |
| Location | `deploy/helm/bobberchat/` |

The chart deploys:
- bobberd Deployment (2 replicas by default)
- bobberd Service (ClusterIP on port 8080)
- PostgreSQL Deployment + PVC + Service (optional, enabled by default)
- Database migration Job
- ConfigMap for backend configuration
- Secret for credentials
- Optional Ingress

## Quick Install

```bash
helm install bobberchat deploy/helm/bobberchat/ \
  --namespace bobberchat --create-namespace \
  --set secrets.jwtSecret="$(openssl rand -base64 32)" \
  --set secrets.postgresPassword="$(openssl rand -base64 16)" \
  --set-file migration.sql=migrations/001_initial_schema.sql
```

## Step-by-Step

### 1. Review Default Values

The full set of configurable values is in `deploy/helm/bobberchat/values.yaml`:

```yaml
replicaCount: 2

image:
  repository: ghcr.io/yourbroduke/bobberchat
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 8080

secrets:
  postgresDsn: "postgres://bobberchat:CHANGE_ME@postgres:5432/bobberchat?sslmode=require"
  jwtSecret: "CHANGE_ME_TO_A_RANDOM_SECRET"
  postgresPassword: "CHANGE_ME"

postgres:
  enabled: true
  storage: 10Gi

migration:
  enabled: true
  sql: ""  # Provide inline or via --set-file
```

### 2. Prepare Secrets

Generate strong values for production:

```bash
JWT_SECRET=$(openssl rand -base64 32)
PG_PASSWORD=$(openssl rand -base64 16)
```

### 3. Install

**Option A: Supply migration SQL via `--set-file`**

```bash
helm install bobberchat deploy/helm/bobberchat/ \
  --namespace bobberchat --create-namespace \
  --set secrets.jwtSecret="$JWT_SECRET" \
  --set secrets.postgresPassword="$PG_PASSWORD" \
  --set "secrets.postgresDsn=postgres://bobberchat:${PG_PASSWORD}@postgres:5432/bobberchat?sslmode=require" \
  --set-file migration.sql=migrations/001_initial_schema.sql
```

**Option B: Create the migration ConfigMap externally**

```bash
kubectl create namespace bobberchat
kubectl create configmap bobberchat-migrations \
  --from-file=migrations/ \
  -n bobberchat

helm install bobberchat deploy/helm/bobberchat/ \
  --namespace bobberchat \
  --set secrets.jwtSecret="$JWT_SECRET" \
  --set secrets.postgresPassword="$PG_PASSWORD" \
  --set "secrets.postgresDsn=postgres://bobberchat:${PG_PASSWORD}@postgres:5432/bobberchat?sslmode=require"
```

### 4. Verify

```bash
# Check release status
helm status bobberchat -n bobberchat

# Wait for pods
kubectl get pods -n bobberchat -w

# Port-forward and test
kubectl port-forward svc/bobberchat-bobberd 8080:8080 -n bobberchat
curl -s http://localhost:8080/v1/health
```

## Configuration Reference

### Application Settings

| Value | Default | Description |
| --- | --- | --- |
| replicaCount | 2 | Number of bobberd replicas |
| image.repository | ghcr.io/yourbroduke/bobberchat | Container image repository |
| image.tag | latest | Image tag |
| image.pullPolicy | IfNotPresent | Image pull policy |
| service.type | ClusterIP | Kubernetes Service type |
| service.port | 8080 | Service port |

### Rate Limits

| Value | Default | Description |
| --- | --- | --- |
| config.rateLimits.perAgent | 100 | Messages per second per agent |
| config.rateLimits.perGroup | 500 | Messages per second per group |
| config.rateLimits.perTag | 200 | Messages per second per tag |
| config.rateLimits.burstFactor | 2.0 | Burst multiplier |

### Infrastructure

| Value | Default | Description |
| --- | --- | --- |
| postgres.enabled | true | Deploy PostgreSQL alongside bobberd |
| postgres.image.tag | 15 | PostgreSQL image version |
| postgres.storage | 10Gi | PVC size for PostgreSQL data |

### Secrets

| Value | Default | Description |
| --- | --- | --- |
| secrets.postgresDsn | (placeholder) | Full PostgreSQL connection string |
| secrets.jwtSecret | (placeholder) | JWT signing secret |
| secrets.postgresPassword | (placeholder) | PostgreSQL password |

### Ingress

| Value | Default | Description |
| --- | --- | --- |
| ingress.enabled | false | Enable Ingress resource |
| ingress.className | "" | Ingress class name |
| ingress.hosts | bobberchat.local | Ingress host configuration |
| ingress.tls | [] | TLS configuration |

Example with Ingress enabled:

```bash
helm install bobberchat deploy/helm/bobberchat/ \
  --namespace bobberchat --create-namespace \
  --set ingress.enabled=true \
  --set ingress.className=nginx \
  --set "ingress.hosts[0].host=bobberchat.example.com" \
  --set "ingress.hosts[0].paths[0].path=/" \
  --set "ingress.hosts[0].paths[0].pathType=Prefix" \
  --set-file migration.sql=migrations/001_initial_schema.sql \
  --set secrets.jwtSecret="$JWT_SECRET" \
  --set secrets.postgresPassword="$PG_PASSWORD"
```

### Migration

| Value | Default | Description |
| --- | --- | --- |
| migration.enabled | true | Run migration Job on install |
| migration.backoffLimit | 3 | Number of retry attempts |
| migration.sql | "" | Inline SQL (or use --set-file) |

## Upgrading

```bash
helm upgrade bobberchat deploy/helm/bobberchat/ \
  --namespace bobberchat \
  --set image.tag=v1.2.3 \
  --reuse-values
```

## Uninstalling

```bash
helm uninstall bobberchat -n bobberchat

# Clean up PVCs if desired
kubectl delete pvc -l app.kubernetes.io/name=bobberchat -n bobberchat
kubectl delete namespace bobberchat
```

## Common Issues

See [troubleshooting.md](troubleshooting.md) for solutions to Helm-specific problems.
