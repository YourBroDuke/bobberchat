# Deploy with Kubernetes (Raw Manifests)

This guide covers deploying BobberChat to Kubernetes using the raw manifest files in `deploy/k8s/`.

## Prerequisites

- Kubernetes cluster (1.24+)
- `kubectl` configured for your cluster
- Container image built and pushed to a registry (default: `ghcr.io/yourbroduke/bobberchat:latest`)
- Migration SQL file available locally (`migrations/001_initial_schema.sql`)

## Manifest Files

The `deploy/k8s/` directory contains:

| File | Resources |
| --- | --- |
| namespace.yml | Namespace `bobberchat` |
| secrets.yml | Secret `bobberchat-secrets` (postgres-dsn, jwt-secret) |
| configmap.yml | ConfigMap `bobberchat-config` (backend.yaml) |
| postgres.yml | PostgreSQL Deployment + PVC + Service + Secret `bobberchat-db-credentials` |
| bobberd.yml | bobberd Deployment + Service + Migration Job + placeholder ConfigMap |

## Step-by-Step

### 1. Create the Namespace

```bash
kubectl apply -f deploy/k8s/namespace.yml
```

### 2. Create the Migrations ConfigMap

The migration Job needs the actual SQL file. Create a ConfigMap from the local migration file:

```bash
kubectl create configmap bobberchat-migrations \
  --from-file=migrations/ \
  -n bobberchat
```

This replaces the placeholder ConfigMap defined at the bottom of `bobberd.yml`. If you have already applied `bobberd.yml`, delete the placeholder first:

```bash
kubectl delete configmap bobberchat-migrations -n bobberchat --ignore-not-found
kubectl create configmap bobberchat-migrations \
  --from-file=migrations/ \
  -n bobberchat
```

### 3. Edit Secrets

Before applying, edit `deploy/k8s/secrets.yml` and `deploy/k8s/postgres.yml` to replace placeholder values:

**In `secrets.yml`:**
- `postgres-dsn`: Set a real PostgreSQL DSN with a strong password
- `jwt-secret`: Set a random string (at least 32 characters)

**In `postgres.yml`:**
- `bobberchat-db-credentials.password`: Must match the password in the DSN above

```bash
# Generate a random JWT secret
openssl rand -base64 32
```

### 4. Apply Manifests in Order

```bash
kubectl apply -f deploy/k8s/namespace.yml
kubectl apply -f deploy/k8s/secrets.yml
kubectl apply -f deploy/k8s/postgres.yml
kubectl apply -f deploy/k8s/configmap.yml
kubectl apply -f deploy/k8s/bobberd.yml
```

Or apply all at once:

```bash
kubectl apply -f deploy/k8s/
```

### 5. Wait for Pods

```bash
kubectl get pods -n bobberchat -w
```

Expected output (after a minute or so):

```
NAME                        READY   STATUS      RESTARTS   AGE
postgres-xxxxx              1/1     Running     0          60s
migrate-xxxxx               0/1     Completed   0          45s
bobberd-xxxxx               1/1     Running     0          30s
bobberd-yyyyy               1/1     Running     0          30s
```

The migration Job should show `Completed`. The bobberd Deployment runs 2 replicas by default.

### 6. Verify Health

Port-forward to test locally:

```bash
kubectl port-forward svc/bobberd 8080:8080 -n bobberchat
```

In another terminal:

```bash
curl -s http://localhost:8080/v1/health
# {"status":"ok"}
```

## Resource Allocation

Default resource requests and limits:

| Component | CPU Request | CPU Limit | Memory Request | Memory Limit |
| --- | --- | --- | --- | --- |
| bobberd | 100m | 500m | 64Mi | 256Mi |
| PostgreSQL | 250m | 1000m | 256Mi | 1Gi |

PostgreSQL uses a 10Gi PersistentVolumeClaim for data storage.

## Scaling

Scale the bobberd Deployment:

```bash
kubectl scale deployment bobberd --replicas=4 -n bobberchat
```

Multiple bobberd replicas are safe because:
- PostgreSQL provides shared state
- Each replica is stateless

Do **not** scale PostgreSQL beyond 1 replica without additional configuration (replication/clustering).

## Monitoring

The bobberd pods expose Prometheus metrics:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/v1/metrics"
```

If you have a Prometheus instance in-cluster, it will auto-discover bobberd pods.

## Updating the Application

```bash
# Build and push a new image
docker build -t ghcr.io/yourbroduke/bobberchat:v1.2.3 .
docker push ghcr.io/yourbroduke/bobberchat:v1.2.3

# Update the deployment
kubectl set image deployment/bobberd bobberd=ghcr.io/yourbroduke/bobberchat:v1.2.3 -n bobberchat
```

## Cleanup

```bash
kubectl delete -f deploy/k8s/
kubectl delete configmap bobberchat-migrations -n bobberchat --ignore-not-found
```

## Common Issues

See [troubleshooting.md](troubleshooting.md) for solutions to Kubernetes-specific problems.
