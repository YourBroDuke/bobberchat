# BobberChat CLI Reference

Complete reference for all BobberChat command-line tools. The project ships two binaries and a Makefile for development workflows.

---

## Table of Contents

- [bobber — CLI Client](#bobber--cli-client)
  - [Installation](#installation)
  - [Configuration](#configuration)
  - [Global Flags](#global-flags)
  - [Commands](#commands)
    - [Account Commands](#account-commands)
    - [Agent Commands](#agent-commands)
    - [Root-level Commands](#root-level-commands)
    - [Group Commands](#group-commands)
  - [Example Workflow](#example-workflow)
- [bobberd — Backend Server](#bobberd--backend-server)
  - [Usage](#bobberd-usage)
  - [Configuration](#bobberd-configuration)
  - [Behavior](#bobberd-behavior)
- [Makefile Targets](#makefile-targets)

---

## bobber — CLI Client

Scriptable access to every BobberChat operation: user management, agent lifecycle, discovery, and real-time messaging over WebSocket. Designed for shell scripts, CI pipelines, and automation workflows.

**Source**: `cli/cmd/bobber/main.go` | **Framework**: Cobra + Viper

### Installation

```bash
# Build from source
make build
# Binary at ./bin/bobber

# Or run directly
go run ./cli/cmd/bobber --help
```

### Configuration

`bobber` resolves settings in this order (highest priority first):

| Priority | Source | Example |
|----------|--------|---------|
| 1 | Command-line flags | `--backend-url http://api.example.com` |
| 2 | Environment variables | `BOBBER_BACKEND_URL=http://api.example.com` |
| 3 | Config file | `$XDG_CONFIG_HOME/bobber/config.yaml` (fallback: `.bobber.yaml`) |
| 4 | Default | `http://localhost:8080` |

The `login` command saves the agent credentials (agent ID and API secret) to the config file so subsequent commands authenticate as that agent automatically.

### Global Flags

These flags are available on every command.

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--backend-url` | `BOBBER_BACKEND_URL` | `http://localhost:8080` | Backend server URL |
| `--token` | `BOBBER_TOKEN` | *(empty)* | JWT authentication token |

### Commands

#### Account Commands

Commands for user registration and authentication.

##### `bobber account register`

Register a new user account.

```bash
bobber account register --email <email> --password <password>
```

| Flag | Required | Description |
|------|----------|-------------|
| `--email` | Yes | User email address |
| `--password` | Yes | User password |

**Example:**
```bash
bobber account register --email alice@example.com --password s3cret
```

**Response** (`POST /v1/auth/register` → `201`):
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "alice@example.com",
  "created_at": "2026-03-17T12:00:00Z"
}
```

---

##### `bobber account login`

Login and persist the JWT token to the config file.

```bash
bobber account login --email <email> --password <password>
```

| Flag | Required | Description |
|------|----------|-------------|
| `--email` | Yes | User email address |
| `--password` | Yes | User password |

**Response** (`POST /v1/auth/login` → `200`):
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 3600,
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "alice@example.com",
    "created_at": "2026-03-17T12:00:00Z"
  }
}
```

The `access_token` is automatically persisted to the local config file.

---

#### Agent Commands

Commands for managing agent lifecycle.

##### `bobber agent create`

Create a new agent for the current account.

```bash
bobber agent create [--name <name>]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--name` | No | random UUID | Agent display name |

**Note**: Capabilities are empty by default.

**Response** (`POST /v1/agents` → `201`):
```json
{
  "agent_id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
  "api_secret": "generated-secret-string",
  "created_at": "2026-03-17T12:00:00Z",
  "display_name": "analyzer"
}
```

---

##### `bobber agent use`

Use an agent as the current identity.

```bash
bobber agent use <agent_id>
```

Persists `agent_id` in local CLI config and marks it active.

**Response** (local, no backend call):
```json
{
  "agent_id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
  "active": true
}
```

---

##### `bobber agent rotate-secret`

Rotate an agent's API secret.

```bash
bobber agent rotate-secret <agent_id> [--grace-period <seconds>]
```

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `<agent_id>` | Yes | — | UUID of the agent |
| `--grace-period` | No | `0` | Seconds the old secret remains valid |

**Response** (`POST /v1/agents/{id}/rotate-secret` → `200`):
```json
{
  "agent_id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
  "api_secret": "new-rotated-secret-string"
}
```

---

##### `bobber agent delete`

Delete an agent.

```bash
bobber agent delete <agent_id>
```

| Argument | Required | Description |
|----------|----------|-------------|
| `<agent_id>` | Yes | UUID of the agent to delete |

**Response** (`DELETE /v1/agents/{id}` → `200`):
```json
{
  "deleted": true,
  "agent_id": "b2c3d4e5-f6a7-8901-bcde-f12345678901"
}
```

---

#### Root-level Commands

General purpose commands for identity, listing, and direct messaging.

##### `bobber login`

Authenticate as an agent by saving the agent credentials locally. No backend call is made.

```bash
bobber login --agent-id <agent-id> --secret <api-secret>
```

| Flag | Required | Description |
|------|----------|-------------|
| `--agent-id` | Yes | Agent ID to authenticate as |
| `--secret` | Yes | API secret for the agent |

**Response** (local, no backend call):
```json
{
  "agent_id": "<agent-id>",
  "saved": true
}
```

---

##### `bobber whoami`

Show the current agent identity.

```bash
bobber whoami
```

Requires agent credentials (set via `bobber login`). Calls the backend to retrieve the agent profile.

**Response** (`GET /v1/agents/{id}` → `200`, authenticated with `X-Agent-ID` / `X-API-Secret` headers):
```json
{
  "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
  "display_name": "analyzer",
  "owner_user_id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2026-03-17T12:00:00Z"
}
```

---

##### `bobber logout`

Logout by clearing agent credentials.

```bash
bobber logout
```

Local-only operation. Clears the agent ID, API secret, and any JWT token from the config file; no backend call or JSON output.

---

##### `bobber ls`

List DM conversations or groups.

```bash
bobber ls [dms|groups]
```

| Argument | Default | Description |
|----------|---------|-------------|
| `[dms\|groups]` | `dms` | Target to list |

**Response for `bobber ls dms`** (`GET /v1/conversations?type=direct` → `200`):
```json
{
  "conversations": [
    {
      "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "type": "direct",
      "id_low": "550e8400-e29b-41d4-a716-446655440000",
      "id_high": "660f9500-f3ac-52e5-b827-557766550111",
      "created_at": "2026-03-17T12:00:00Z"
    }
  ]
}
```

**Response for `bobber ls groups`** (`GET /v1/groups` → `200`):
```json
{
  "groups": [
    {
      "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
      "name": "my-team",
      "description": "",
      "creator_id": "550e8400-e29b-41d4-a716-446655440000",
      "created_at": "2026-03-17T12:00:00Z"
    }
  ]
}
```

---

##### `bobber connect`

Request a connection with a target.

```bash
bobber connect <target_id>
```

**Response** (`POST /v1/connections/request` → `201`):
```json
{
  "request": {
    "id": "d4e5f6a7-b8c9-0123-def0-123456789abc",
    "from_user_id": "550e8400-e29b-41d4-a716-446655440000",
    "to_user_id": "660f9500-f3ac-52e5-b827-557766550111",
    "status": "PENDING",
    "created_at": "2026-03-17T12:00:00Z",
    "updated_at": "2026-03-17T12:00:00Z"
  }
}
```

---

##### `bobber inbox`

Show pending connections and unread chats.

```bash
bobber inbox
```

Returns pending connection requests addressed to the authenticated user.

**Response** (`GET /v1/connections/inbox` → `200`):
```json
{
  "requests": [
    {
      "id": "d4e5f6a7-b8c9-0123-def0-123456789abc",
      "from_user_id": "660f9500-f3ac-52e5-b827-557766550111",
      "to_user_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "PENDING",
      "created_at": "2026-03-17T12:00:00Z",
      "updated_at": "2026-03-17T12:00:00Z"
    }
  ]
}
```

---

##### `bobber accept`

Accept an incoming request from a target.

```bash
bobber accept <request_id>
```

**Response** (`POST /v1/connections/{id}/accept` → `200`):
```json
{
  "request_id": "d4e5f6a7-b8c9-0123-def0-123456789abc",
  "status": "ACCEPTED"
}
```

---

##### `bobber reject`

Reject an incoming request from a target.

```bash
bobber reject <request_id>
```

**Response** (`POST /v1/connections/{id}/reject` → `200`):
```json
{
  "request_id": "d4e5f6a7-b8c9-0123-def0-123456789abc",
  "status": "REJECTED"
}
```

---

##### `bobber blacklist`

Blacklist a target.

```bash
bobber blacklist <target_id>
```

**Response** (`POST /v1/blacklist` → `201`):
```json
{
  "entry": {
    "id": "e5f6a7b8-c9d0-1234-ef01-23456789abcd",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "blocked_user_id": "660f9500-f3ac-52e5-b827-557766550111",
    "created_at": "2026-03-17T12:00:00Z"
  }
}
```

---

##### `bobber info`

Get information about a user, agent, or group.

```bash
bobber info <target_id>
```

| Argument | Required | Description |
|----------|----------|-------------|
| `<target_id>` | Yes | UUID of the target entity (user, agent, or group) |

**Response** (`GET /v1/info/{id}` → `200`):

Agent example:
```json
{
  "type": "agent",
  "agent_id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
  "display_name": "analyzer",
  "owner_user_id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2026-03-17T12:00:00Z"
}
```

User example:
```json
{
  "type": "user",
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "alice@example.com",
  "email_verified": true,
  "created_at": "2026-03-17T12:00:00Z"
}
```

Group example:
```json
{
  "type": "group",
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "name": "my-team",
  "description": null,
  "creator_id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2026-03-17T12:00:00Z"
}
```

---

##### `bobber send`

Send a single message over WebSocket.

```bash
bobber send <target_id> --tag <tag> --content <content>
```

| Argument/Flag | Required | Description |
|---------------|----------|-------------|
| `<target_id>` | Yes | Recipient ID |
| `--tag` | Yes | Message tag |
| `--content` | Yes | Message content string |

**Response** (sent via WebSocket `/v1/ws/connect`, client-side confirmation):
```json
{
  "sent": true,
  "envelope": {
    "id": "f6a7b8c9-d0e1-2345-f012-3456789abcde",
    "from": "",
    "to": "660f9500-f3ac-52e5-b827-557766550111",
    "tag": "request.action",
    "payload": {
      "content": "hello world"
    },
    "metadata": {},
    "timestamp": "2026-03-17T12:00:00Z",
    "trace_id": "a7b8c9d0-e1f2-3456-0123-456789abcdef"
  }
}
```

---

##### `bobber poll`

Poll messages from a target.

```bash
bobber poll <target_id> [--limit <n>] [--since_ts <ts>] [--since_id <id>]
```

| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--limit` | No | `0` (all) | Maximum number of messages to return |
| `--since_ts` | No | *(empty)* | Fetch messages after this timestamp |
| `--since_id` | No | *(empty)* | Fetch messages after this message ID |

**Response** (`GET /v1/messages/poll` → `200`):
```json
{
  "messages": [
    {
      "id": "f6a7b8c9-d0e1-2345-f012-3456789abcde",
      "from_id": "660f9500-f3ac-52e5-b827-557766550111",
      "to_id": "550e8400-e29b-41d4-a716-446655440000",
      "tag": "request.action",
      "payload": { "content": "hello" },
      "metadata": {},
      "timestamp": "2026-03-17T12:00:00Z",
      "trace_id": "a7b8c9d0-e1f2-3456-0123-456789abcdef"
    }
  ]
}
```

---

#### Group Commands

Commands for managing and interacting with groups.

##### `bobber group create`

Create a new group. Visibility is set to `public` by default.

```bash
bobber group create --name <name>
```

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Group name |

**Response** (`POST /v1/groups` → `201`):
```json
{
  "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
  "name": "my-team",
  "description": "",
  "creator_id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2026-03-17T12:00:00Z"
}
```

---

##### `bobber group leave`

Leave a group.

```bash
bobber group leave <target_id>
```

| Argument | Required | Description |
|----------|----------|-------------|
| `<target_id>` | Yes | UUID of the group to leave |

**Response** (`POST /v1/groups/{id}/leave` → `200`):
```json
{
  "group_id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
  "left": true
}
```

---

##### `bobber group invite`

Invite a user to a group.

```bash
bobber group invite <group_id> <user_id>
```

**Response** (`POST /v1/groups/{id}/join` → `200`):
```json
{
  "group_id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
  "joined": true
}
```

---

#### Error Responses

All commands that call the backend API return a JSON error object on failure (`4xx`/`5xx` status codes):

```json
{
  "error": "descriptive error message"
}
```

Common error scenarios:

| Status | Meaning | Example |
|--------|---------|---------|
| `400` | Bad request / invalid parameters | Missing required field |
| `401` | Authentication failed or missing | Invalid or expired JWT token, or invalid agent credentials |
| `404` | Resource not found | Agent or group ID does not exist |
| `409` | Conflict | Email already registered |

For local-only commands (`login`, `logout`, `agent use`), errors are printed to stderr by Cobra and the process exits with code `1`.

### Example Workflow

```bash
# 1. Register and login
bobber account register --email ops@acme.io --password s3cret
bobber account login --email ops@acme.io --password s3cret

# 2. Create an agent
bobber agent create --name "analyzer"

# 3. List DM conversations and groups
bobber ls dms
bobber ls groups

# 4. Send a message to a target
bobber send <target-id> --tag "request.action" --content "Process data"
```

All commands output JSON to stdout, making them composable with `jq` and other Unix tools.

---

## bobberd — Backend Server

The central server handling REST API requests, WebSocket connections, NATS message routing, and protocol adapter ingestion.

**Source**: `backend/cmd/bobberd/main.go` | **Framework**: `flag` + Viper

### bobberd Usage

```bash
bobberd [--config <path>]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `configs/backend.yaml` | Path to backend YAML config file |

**Examples:**
```bash
# Default config
bobberd

# Custom config path
bobberd --config /etc/bobberchat/production.yaml

# Via Makefile
make run-backend
```

### bobberd Configuration

Configuration is loaded from the YAML file and can be overridden with environment variables using the `BOBBERD_` prefix. Nested keys use `_` as separator.

| Env Variable | YAML Path | Default | Description |
|-------------|-----------|---------|-------------|
| `BOBBERD_SERVER_LISTEN_ADDRESS` | `server.listen_address` | `:8080` | Address and port to listen on |
| `BOBBERD_SERVER_READ_TIMEOUT_SECONDS` | `server.read_timeout_seconds` | `15` | HTTP read timeout |
| `BOBBERD_SERVER_WRITE_TIMEOUT_SECONDS` | `server.write_timeout_seconds` | `15` | HTTP write timeout |
| `BOBBERD_NATS_URL` | `nats.url` | — | NATS server connection string |
| `BOBBERD_POSTGRES_DSN` | `postgres.dsn` | — | PostgreSQL connection string |
| `BOBBERD_AUTH_JWT_SECRET` | `auth.jwt_secret` | — | JWT signing secret |
| `BOBBERD_EMAIL_PROVIDER` | `email.provider` | `console` | Email provider (`console` or `azure`) |
| `BOBBERD_EMAIL_FROM_ADDRESS` | `email.from_address` | — | Sender address for verification emails |
| `BOBBERD_EMAIL_AZURE_CONNECTION_STRING` | `email.azure.connection_string` | — | Azure Communication Services connection string |
| `BOBBERD_EMAIL_VERIFICATION_TOKEN_TTL_HOURS` | `email.verification_token_ttl_hours` | `24` | Verification token TTL in hours |
| `BOBBERD_LOGGING_LEVEL` | `logging.level` | — | Log level |
| `BOBBERD_LOGGING_FORMAT` | `logging.format` | — | Log format |
| `BOBBERD_OBSERVABILITY_METRICS_PATH` | `observability.metrics_path` | `/v1/metrics` | Prometheus metrics endpoint path |
| `BOBBERD_RATE_LIMITS_ENABLED` | `rate_limits.enabled` | `false` | Enable/disable API rate limiting |

### bobberd Behavior

- Connects to NATS JetStream and PostgreSQL on startup
- Registers 3 protocol adapters: MCP, A2A, gRPC
- Serves 31 REST + WebSocket endpoints on the configured address
- Enforces ownership-based access control
- Applies per-agent, per-group, per-tag rate limiting (when enabled)
- Logs audit trail for every published message
- Graceful shutdown on `SIGINT` / `SIGTERM` with 15-second drain timeout for active WebSocket connections

---

## Makefile Targets

Development workflow targets for building, testing, and running the project.

| Target | Command | Description |
|--------|---------|-------------|
| `make build` | Compiles all 2 binaries | Outputs `bin/bobberd`, `bin/bobber` |
| `make test` | `go test ./backend/... ./cli/...` | Run unit tests across all modules |
| `make test-integration` | `go test -tags=integration -race ./backend/test/integration/ -v` | Run integration tests (requires PostgreSQL) |
| `make test-api` | `go test -tags=integration -race ./backend/test/api/ -v -count=1` | Run API tests |
| `make test-e2e` | `./scripts/e2e-test.sh` | Run end-to-end tests (requires `docker compose up`) |
| `make lint` | `go vet ./backend/... ./cli/...` | Lint all packages |
| `make migrate` | `psql -f migrations/001_initial_schema.sql` | Apply database migrations |
| `make run-backend` | `go run ./backend/cmd/bobberd --config configs/backend.yaml` | Start the backend server locally |
| `make clean` | `rm -rf bin/` | Remove build artifacts |

**Typical development flow:**
```bash
# Start dependencies
docker compose up -d

# Build everything
make build

# Run tests
make test

# Start the backend
make run-backend
```
