# Deploy with Docker Compose

This guide covers deploying BobberChat using Docker Compose for development and staging environments.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose v2+
- The BobberChat repository cloned locally

## Architecture

Docker Compose starts four services:

| Service | Image | Purpose | Exposed Port |
| --- | --- | --- | --- |
| nats | nats:2.10 | Message broker with JetStream | 4222 (client), 8222 (monitoring) |
| postgres | postgres:15 | Persistent storage | 5432 |
| init-db | postgres:15 | Runs schema migration then exits | none |
| bobberd | Built from Dockerfile | Application server | 8080 |

## Quick Start

```bash
# Clean start (removes previous data)
docker compose down -v

# Build and start all services, wait for health checks
docker compose up -d --build --wait
```

## Step-by-Step

### 1. Start the Stack

```bash
docker compose up -d --build --wait
```

The `--wait` flag blocks until all services report healthy. The startup order is:

1. `nats` and `postgres` start in parallel
2. Both pass health checks (NATS: `--help` exit code, Postgres: `pg_isready`)
3. `init-db` runs `migrations/001_initial_schema.sql` against Postgres, then exits
4. `bobberd` starts after `init-db` completes successfully

### 2. Verify Health

```bash
curl -s http://localhost:8080/v1/health
```

Expected response:

```json
{"status":"ok"}
```

### 3. Check NATS Monitoring

NATS exposes monitoring endpoints on port 8222:

```bash
# Server health
curl -s http://localhost:8222/healthz

# JetStream status
curl -s http://localhost:8222/jsz

# Active connections
curl -s http://localhost:8222/connz
```

### 4. View Logs

```bash
# All services
docker compose logs -f

# Single service
docker compose logs -f bobberd
```

## Environment Variables

The following environment variables are set in `docker-compose.yml` for the `bobberd` service:

| Variable | Default Value | Description |
| --- | --- | --- |
| BOBBERD_NATS_URL | nats://nats:4222 | NATS connection string (uses Docker DNS) |
| BOBBERD_POSTGRES_DSN | postgres://bobberchat:bobberchat@postgres:5432/bobberchat?sslmode=disable | PostgreSQL connection string |
| BOBBERD_SERVER_LISTEN_ADDRESS | :8080 | Listen address inside the container |

Additional variables can be added to override values in `configs/backend.yaml`. The Viper prefix is `BOBBERD` and nested keys use `_` as separator (e.g., `auth.jwt_secret` becomes `BOBBERD_AUTH_JWT_SECRET`).

## Port Mapping

| Host Port | Container Port | Service |
| --- | --- | --- |
| 8080 | 8080 | bobberd (REST API + WebSocket) |
| 4222 | 4222 | NATS client connections |
| 8222 | 8222 | NATS monitoring HTTP |
| 5432 | 5432 | PostgreSQL |

## Data Persistence

By default, Docker Compose does **not** define named volumes for Postgres data. Data is lost when running `docker compose down -v`. For persistent data across restarts:

1. Add a named volume to `docker-compose.yml`:
   ```yaml
   volumes:
     pgdata:

   services:
     postgres:
       volumes:
         - pgdata:/var/lib/postgresql/data
   ```
2. Use `docker compose down` (without `-v`) to preserve data.
3. Use `docker compose down -v` only for a clean reset.

## Rebuilding

After code changes:

```bash
# Rebuild only the bobberd image
docker compose up -d --build bobberd

# Full rebuild from scratch
docker compose down -v && docker compose up -d --build --wait
```

## Stopping

```bash
# Stop and remove containers (keep data)
docker compose down

# Stop, remove containers, and delete volumes (clean slate)
docker compose down -v
```

## Common Issues

See [troubleshooting.md](troubleshooting.md) for solutions to frequent Docker Compose problems.
