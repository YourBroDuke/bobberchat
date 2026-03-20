# Deploy Locally (Development)

This guide covers running BobberChat natively on your development machine with Go, using Docker only for infrastructure dependencies (PostgreSQL).

## Prerequisites

- Go 1.25+
- Docker Engine 20.10+ and Docker Compose v2+ (for PostgreSQL)
- `psql` client (for running migrations)
- The BobberChat repository cloned locally

## Step-by-Step

### 1. Start Infrastructure Dependencies

Start only PostgreSQL using Docker Compose:

```bash
docker compose up -d postgres
```

Wait for both services to be healthy:

```bash
docker compose ps
```

`postgres` should show `healthy` status.

### 2. Run Database Migration

Apply the schema to PostgreSQL:

```bash
make migrate
```

This runs `psql` against `localhost:5432` using the default credentials (`bobberchat`/`bobberchat`). Override with environment variables if needed:

```bash
PGHOST=localhost PGUSER=bobberchat PGPASSWORD=bobberchat PGDB=bobberchat make migrate
```

### 3. Start the Backend

```bash
make run-backend
```

This executes `go run ./backend/cmd/bobberd --config configs/backend.yaml`. The default config connects to `postgres://bobberchat:bobberchat@localhost:5432/bobberchat?sslmode=disable`.

Verify:

```bash
curl -s http://localhost:8080/v1/health
# {"status":"ok"}
```

### 4. Build Binaries (Optional)

To compile all binaries (`bobberd`, `bobber`) into the `bin/` directory:

```bash
make build
```

Then run directly:

```bash
./bin/bobberd --config configs/backend.yaml
```

## Configuration

The default configuration file is `configs/backend.yaml`. Key settings for local development:

| Setting | Default | Notes |
| --- | --- | --- |
| server.listen_address | :8080 | HTTP server bind address |
| postgres.dsn | postgres://bobberchat:bobberchat@localhost:5432/bobberchat?sslmode=disable | Matches Docker Compose PostgreSQL |
| auth.jwt_secret | change-me | Acceptable for local dev |
| rate_limits.enabled | true | Can disable for testing |

Override any setting via environment variables with the `BOBBERD_` prefix:

```bash
BOBBERD_AUTH_JWT_SECRET=my-dev-secret go run ./backend/cmd/bobberd --config configs/backend.yaml
```

## Makefile Targets

| Target | Command | Description |
| --- | --- | --- |
| make build | `go build -o bin/ ./backend/cmd/bobberd ./cli/cmd/bobber` | Compile all binaries |
| make test | `go test ./backend/... ./cli/...` | Run unit tests |
| make lint | `go vet ./backend/... ./cli/...` | Run static analysis |
| make migrate | `psql -f migrations/001_initial_schema.sql` | Apply database schema |
| make run-backend | `go run ./backend/cmd/bobberd` | Start backend server |
| make clean | `rm -rf bin/` | Remove build artifacts |

## Running Tests

### Unit Tests

```bash
make test
```

### Integration Tests

Integration tests require a running PostgreSQL instance:

```bash
go test -v ./backend/test/integration/...
```

### End-to-End Tests

E2E tests require the full stack running via Docker Compose:

```bash
docker compose down -v && docker compose up -d --build --wait
./scripts/e2e-test.sh
```

The E2E script runs 27 API checks covering registration/login, agent CRUD, groups, adapter checks, metrics, and negative auth/error-path validation.

## Stopping

```bash
# Stop the Go processes (Ctrl+C in their terminals)

# Stop infrastructure
docker compose down

# Stop infrastructure and delete data
docker compose down -v
```

## Common Issues

See [troubleshooting.md](troubleshooting.md) for solutions to local development problems.
