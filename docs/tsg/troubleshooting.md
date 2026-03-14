# Troubleshooting Guide

Common issues and solutions for BobberChat deployments.

## Docker Compose Issues

### "user already exists" or 400 on /v1/auth/register

**Symptom**: Registration returns HTTP 400 with an error indicating the user or email already exists.

**Cause**: The database contains data from a previous run. The E2E test script and manual testing create a user that persists across container restarts.

**Fix**:
```bash
docker compose down -v && docker compose up -d --build --wait
```

The `-v` flag removes volumes, giving you a clean database.

---

### NATS container keeps restarting (health check flapping)

**Symptom**: The NATS container shows `unhealthy` or enters a restart loop.

**Cause**: Earlier versions of the health check used `wget` which is not available in the NATS Docker image. The correct health check uses `/nats-server --help`.

**Fix**: Ensure `docker-compose.yml` has this health check for NATS:

```yaml
healthcheck:
  test: ["CMD", "/nats-server", "--help"]
  interval: 2s
  timeout: 5s
  retries: 10
  start_period: 5s
```

The `start_period` is important to avoid premature failure detection.

---

### bobberd container won't start

**Symptom**: The bobberd container exits immediately or shows connection errors in logs.

**Cause**: Usually means NATS or PostgreSQL is not ready, or the migration has not completed.

**Fix**:
1. Check that `init-db` completed successfully:
   ```bash
   docker compose logs init-db
   ```
2. Check the bobberd logs:
   ```bash
   docker compose logs bobberd
   ```
3. Ensure the `depends_on` conditions are correct in `docker-compose.yml`:
   ```yaml
   bobberd:
     depends_on:
       nats:
         condition: service_healthy
       postgres:
         condition: service_healthy
       init-db:
         condition: service_completed_successfully
   ```

---

### Port conflict on 8080, 4222, or 5432

**Symptom**: `docker compose up` fails with "port is already allocated".

**Cause**: Another process is using the port.

**Fix**:
```bash
# Find what's using the port
lsof -i :8080

# Either stop the conflicting process or remap ports in docker-compose.yml
# Example: map bobberd to host port 9090 instead
ports:
  - "9090:8080"
```

## API Issues

### WebSocket returns 401

**Symptom**: Connecting to `/v1/ws/connect` returns HTTP 401 Unauthorized.

**Cause**: Missing or invalid JWT token. The WebSocket endpoint accepts authentication via either:
- Query parameter: `?token=<JWT>`
- Header: `Authorization: Bearer <JWT>`

**Fix**:
1. Verify your token is valid and not expired (default TTL: 3600 seconds):
   ```bash
   # Register and login to get a fresh token
   curl -s -X POST http://localhost:8080/v1/auth/register \
     -H "Content-Type: application/json" \
     -d '{"email":"test@test.com","password":"pass1234","tenant_id":"550e8400-e29b-41d4-a716-446655440000"}'

   curl -s -X POST http://localhost:8080/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"email":"test@test.com","password":"pass1234"}'
   ```
2. Use the token from the login response.

---

### 400 Bad Request with "unknown field"

**Symptom**: API returns 400 with a message about unknown fields in the JSON body.

**Cause**: The server uses `json.Decoder` with `DisallowUnknownFields()`. Any JSON key not matching a struct field is rejected.

**Fix**: Ensure your JSON request body matches the expected schema exactly. Remove any extra fields. Check the OpenAPI spec at `api/openapi/openapi.yaml` for the correct request format.

---

### Agent authentication fails with X-Agent-ID / X-API-Secret

**Symptom**: Requests using agent credentials return 401 or 403.

**Cause**: Agent endpoints accept authentication via two methods:
1. JWT Bearer token (`Authorization: Bearer <token>`)
2. Agent credentials (`X-Agent-ID` + `X-API-Secret` headers)

For method 2, the `X-API-Secret` must be the raw secret returned at agent creation time, not the hashed version.

**Fix**: Use the exact `api_secret` value returned from `POST /v1/agents`. This value is only shown once at creation time.

## Kubernetes Issues

### Migration Job fails

**Symptom**: The `migrate` Job shows `Error` or `BackoffLimitExceeded`.

**Cause**: Usually the migration ConfigMap contains the placeholder text instead of actual SQL.

**Fix**:
1. Check the migration Job logs:
   ```bash
   kubectl logs job/migrate -n bobberchat
   ```
2. Recreate the ConfigMap from the actual migration file:
   ```bash
   kubectl delete configmap bobberchat-migrations -n bobberchat --ignore-not-found
   kubectl create configmap bobberchat-migrations --from-file=migrations/ -n bobberchat
   ```
3. Delete and recreate the Job:
   ```bash
   kubectl delete job migrate -n bobberchat
   kubectl apply -f deploy/k8s/bobberd.yml
   ```

---

### Pods stuck in CrashLoopBackOff

**Symptom**: bobberd pods repeatedly restart.

**Cause**: Typically a misconfigured secret or unreachable dependency.

**Fix**:
1. Check pod logs:
   ```bash
   kubectl logs deployment/bobberd -n bobberchat --previous
   ```
2. Verify secrets are correctly set:
   ```bash
   kubectl get secret bobberchat-secrets -n bobberchat -o yaml
   ```
3. Verify NATS and PostgreSQL services are running:
   ```bash
   kubectl get pods -n bobberchat
   kubectl get svc -n bobberchat
   ```

---

### PersistentVolumeClaim stuck in Pending

**Symptom**: The `postgres-data` PVC shows `Pending` status.

**Cause**: No StorageClass is available or the default StorageClass cannot provision the volume.

**Fix**:
1. Check available StorageClasses:
   ```bash
   kubectl get storageclass
   ```
2. Set the StorageClass explicitly in `postgres.yml`:
   ```yaml
   spec:
     storageClassName: standard  # or your cluster's StorageClass
   ```

## Helm Issues

### Helm install fails with "migration.sql" empty

**Symptom**: The migration Job runs but does nothing because the SQL is empty.

**Cause**: The `migration.sql` value was not provided during install.

**Fix**: Supply the SQL file:
```bash
helm upgrade bobberchat deploy/helm/bobberchat/ \
  --namespace bobberchat \
  --set-file migration.sql=migrations/001_initial_schema.sql \
  --reuse-values
```

## make migrate fails

**Symptom**: `make migrate` reports connection refused or authentication failure.

**Cause**: The `psql` client cannot connect to PostgreSQL, or the default credentials don't match.

**Fix**:
1. Ensure PostgreSQL is running:
   ```bash
   docker compose ps postgres
   ```
2. Override credentials if they differ from defaults:
   ```bash
   PGHOST=localhost PGUSER=bobberchat PGPASSWORD=bobberchat PGDB=bobberchat make migrate
   ```
3. If using a remote database, set all PG environment variables accordingly.

## TUI Issues

### TUI shows "connecting..." indefinitely

**Symptom**: The TUI starts but never shows agents or messages.

**Cause**: The WebSocket connection cannot be established. Common reasons:
- Backend is not running
- Wrong `--backend-url`
- Invalid or expired token

**Fix**:
1. Verify the backend is healthy:
   ```bash
   curl -s http://localhost:8080/v1/health
   ```
2. Check your token is valid (login again if expired)
3. Ensure the URL scheme matches (http not https for local dev)

---

### TUI shows agents but no messages

**Symptom**: The left pane shows agents, but the center pane is empty.

**Cause**: No messages have been sent yet, or the WebSocket is not subscribed to the correct tenant.

**Fix**:
1. Send a test message via the API (see [manual-testing.md](manual-testing.md))
2. Verify the `--tenant-id` flag matches the tenant used when creating agents and messages
