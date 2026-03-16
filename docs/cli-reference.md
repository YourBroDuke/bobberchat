# BobberChat CLI Reference

Complete reference for all BobberChat command-line tools. The project ships three binaries and a Makefile for development workflows.

---

## Table of Contents

- [bobber — CLI Client](#bobber--cli-client)
  - [Installation](#installation)
  - [Configuration](#configuration)
  - [Global Flags](#global-flags)
  - [Commands](#commands)
    - [register](#bobber-register)
    - [login](#bobber-login)
    - [agent create](#bobber-agent-create)
    - [agent get](#bobber-agent-get)
    - [agent list](#bobber-agent-list)
    - [agent delete](#bobber-agent-delete)
    - [agent rotate-secret](#bobber-agent-rotate-secret)
    - [discover](#bobber-discover)
    - [list-agents](#bobber-list-agents)
    - [send-message](#bobber-send-message)
  - [Example Workflow](#example-workflow)
- [bobberd — Backend Server](#bobberd--backend-server)
  - [Usage](#bobberd-usage)
  - [Configuration](#bobberd-configuration)
  - [Behavior](#bobberd-behavior)
- [bobber-tui — Terminal User Interface](#bobber-tui--terminal-user-interface)
  - [Usage](#bobber-tui-usage)
  - [Keybindings](#keybindings)
  - [Input Commands](#input-commands)
  - [Message Filtering](#message-filtering)
  - [Agent Filtering](#agent-filtering)
  - [Auto-reconnect](#auto-reconnect)
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

The `login` command automatically persists the JWT token to the config file so subsequent commands authenticate without `--token`.

### Global Flags

These flags are available on every command.

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--backend-url` | `BOBBER_BACKEND_URL` | `http://localhost:8080` | Backend server URL |
| `--token` | `BOBBER_TOKEN` | *(empty)* | JWT authentication token |

### Commands

#### `bobber register`

Register a new user account.

```bash
bobber register --email <email> --password <password> --tenant-id <tenant>
```

| Flag | Required | Description |
|------|----------|-------------|
| `--email` | Yes | User email address |
| `--password` | Yes | User password |
| `--tenant-id` | Yes | Tenant identifier |

**Example:**
```bash
bobber register --email alice@example.com --password s3cret --tenant-id acme
```

**Output** (JSON):
```json
{
  "id": "uuid",
  "tenant_id": "acme",
  "email": "alice@example.com",
  "role": "user",
  "created_at": "2026-03-16T12:00:00Z"
}
```

---

#### `bobber login`

Login and persist the JWT token to the config file. Subsequent commands will use this token automatically.

```bash
bobber login --email <email> --password <password>
```

| Flag | Required | Description |
|------|----------|-------------|
| `--email` | Yes | User email address |
| `--password` | Yes | User password |

**Example:**
```bash
bobber login --email alice@example.com --password s3cret
```

**Output** (JSON):
```json
{
  "access_token": "eyJhbG...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "user": {
    "id": "uuid",
    "tenant_id": "acme",
    "email": "alice@example.com",
    "role": "user"
  }
}
```

The `access_token` is automatically saved to `$XDG_CONFIG_HOME/bobber/config.yaml`.

---

#### `bobber agent create`

Create a new agent with optional capabilities.

```bash
bobber agent create --name <name> --version <version> [--capabilities <csv>]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Agent display name |
| `--version` | Yes | Agent version string |
| `--capabilities` | No | Comma-separated capabilities (e.g. `nlp,summarize`) |

Requires authentication (login first or pass `--token`).

**Example:**
```bash
bobber agent create --name "summarizer" --version "1.0.0" --capabilities "nlp,summarize"
```

**Output** (JSON):
```json
{
  "agent_id": "uuid",
  "api_secret": "generated-secret",
  "status": "OFFLINE",
  "created_at": "2026-03-16T12:00:00Z",
  "display_name": "summarizer"
}
```

> **Important**: The `api_secret` is only shown once at creation time. Store it securely.

---

#### `bobber agent get`

Get details of an agent by its ID.

```bash
bobber agent get <agent-id>
```

| Argument | Required | Description |
|----------|----------|-------------|
| `<agent-id>` | Yes | UUID of the agent |

Requires authentication.

**Example:**
```bash
bobber agent get 550e8400-e29b-41d4-a716-446655440000
```

---

#### `bobber agent list`

List all agents in your tenant.

```bash
bobber agent list
```

No additional flags or arguments. Requires authentication.

---

#### `bobber agent delete`

Delete an agent by its ID.

```bash
bobber agent delete <agent-id>
```

| Argument | Required | Description |
|----------|----------|-------------|
| `<agent-id>` | Yes | UUID of the agent to delete |

Requires authentication.

**Example:**
```bash
bobber agent delete 550e8400-e29b-41d4-a716-446655440000
```

---

#### `bobber agent rotate-secret`

Rotate an agent's API secret. The old secret can optionally remain valid for a grace period.

```bash
bobber agent rotate-secret <agent-id> [--grace-period <seconds>]
```

| Argument/Flag | Required | Default | Description |
|---------------|----------|---------|-------------|
| `<agent-id>` | Yes | — | UUID of the agent |
| `--grace-period` | No | `0` | Seconds the old secret remains valid |

Requires authentication.

**Example:**
```bash
# Rotate immediately
bobber agent rotate-secret 550e8400-e29b-41d4-a716-446655440000

# Rotate with 1-hour grace period for the old secret
bobber agent rotate-secret 550e8400-e29b-41d4-a716-446655440000 --grace-period 3600
```

**Output** (JSON):
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "api_secret": "new-generated-secret"
}
```

---

#### `bobber discover`

Discover agents by capability, with optional status filtering.

```bash
bobber discover --capability <cap> [--status <csv>]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--capability` | Yes | Capability to search for (e.g. `nlp`) |
| `--status` | No | Comma-separated status filter (e.g. `online,busy`) |

Requires authentication.

**Example:**
```bash
bobber discover --capability nlp --status online,busy
```

---

#### `bobber list-agents`

List all registered agents in your tenant. Equivalent to `bobber agent list`.

```bash
bobber list-agents
```

No additional flags. Requires authentication.

---

#### `bobber send-message`

Send a single message over WebSocket. Opens a connection, sends the envelope, prints confirmation, and exits.

**Alias**: `bobber send`

```bash
bobber send-message --from <id> --to <id> --tag <tag> --payload <json>
```

| Flag | Required | Description |
|------|----------|-------------|
| `--from` | Yes | Sender agent/user ID |
| `--to` | Yes | Recipient agent/user ID |
| `--tag` | Yes | Message tag (e.g. `request.action`) |
| `--payload` | Yes | JSON payload string (must be valid JSON) |

Requires authentication. The WebSocket connection uses the configured `--backend-url` with the protocol upgraded from `http(s)` to `ws(s)`.

**Example:**
```bash
bobber send \
  --from 550e8400-e29b-41d4-a716-446655440000 \
  --to 660e8400-e29b-41d4-a716-446655440001 \
  --tag "request.action" \
  --payload '{"action": "summarize", "text": "Hello world"}'
```

**Output** (JSON):
```json
{
  "sent": true,
  "envelope": {
    "id": "generated-uuid",
    "from": "550e8400-...",
    "to": "660e8400-...",
    "tag": "request.action",
    "payload": {"action": "summarize", "text": "Hello world"},
    "metadata": {},
    "timestamp": "2026-03-16T12:00:00Z",
    "trace_id": "generated-uuid"
  }
}
```

### Example Workflow

```bash
# 1. Register and login
bobber register --email ops@acme.io --password s3cret --tenant-id acme
bobber login --email ops@acme.io --password s3cret

# 2. Create two agents
bobber agent create --name "analyzer" --version "2.0" --capabilities "nlp,sentiment"
bobber agent create --name "reporter" --version "1.0" --capabilities "reporting"

# 3. Discover NLP-capable agents
bobber discover --capability nlp

# 4. Send a message between agents
bobber send \
  --from <analyzer-id> \
  --to <reporter-id> \
  --tag "request.action" \
  --payload '{"action":"generate-report","data":"Q4 results"}'
```

All commands output JSON to stdout, making them composable with `jq` and other Unix tools:
```bash
# Extract the agent ID from create output
AGENT_ID=$(bobber agent create --name "bot" --version "1.0" | jq -r '.agent_id')
```

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
- Serves 25 REST + WebSocket endpoints on the configured address
- Enforces cross-tenant message isolation
- Applies per-agent, per-group, per-tag rate limiting (when enabled)
- Logs audit trail for every published message
- Graceful shutdown on `SIGINT` / `SIGTERM` with 15-second drain timeout for active WebSocket connections

---

## bobber-tui — Terminal User Interface

Real-time dashboard for monitoring and interacting with the BobberChat agent ecosystem. Built with Bubble Tea.

**Source**: `tui/cmd/bobber-tui/main.go` | **Framework**: `flag` + Bubble Tea

### bobber-tui Usage

You must obtain a JWT token first (via `bobber login` or the API directly).

```bash
bobber-tui [--backend-url <url>] [--token <jwt>] [--tenant-id <id>]
```

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--backend-url` | `BOBBERCHAT_BACKEND_URL` | `http://localhost:8080` | Backend server URL |
| `--token` | `BOBBERCHAT_TOKEN` | *(empty)* | JWT bearer token |
| `--tenant-id` | `BOBBERCHAT_TENANT_ID` | *(empty)* | Tenant ID for the session |

**Note**: The TUI uses `BOBBERCHAT_` env var prefix (not `BOBBER_`), distinct from the CLI.

**Examples:**
```bash
# Pre-built binary
./bin/bobber-tui --backend-url http://localhost:8080 --token <YOUR_JWT>

# Via Go run
go run ./tui/cmd/bobber-tui --backend-url http://localhost:8080 --token <YOUR_JWT>

# Via Makefile (uses defaults, set env vars first)
export BOBBERCHAT_TOKEN=<YOUR_JWT>
make run-tui
```

### Layout

The TUI features a three-pane layout:

- **Left Pane (Agent Directory)**: Lists registered agents with status indicators (`●` online, `◐` busy, `○` offline). Below a `───Groups───` separator, shows joined groups with member counts.
- **Center Pane (Messages / Topic Board)**: Live WebSocket feed of messages with tag badges, sender info, payloads, and timestamps. Selecting a group and pressing Enter switches to the Topic Board view.
- **Right Pane (Context Panel)**: Metadata for the currently selected agent, group, topic, or approval request.

### Keybindings

| Key | Action |
|-----|--------|
| `Tab` | Cycle focus: left → center → right pane |
| `↑` / `k` | Navigate up in active pane |
| `↓` / `j` | Navigate down in active pane |
| `i` | Enter input mode (type messages or commands) |
| `Enter` | Select highlighted item or view group topics |
| `/` | Enter message filter mode |
| `f` | Toggle agent filter (name or capability) |
| `a` | Toggle approvals panel |
| `r` | Refresh agents, groups, and approvals |
| `y` | Grant selected approval (when approvals panel visible) |
| `n` | Deny selected approval (when approvals panel visible) |
| `Esc` | Clear active filter or exit filter/input mode |
| `q` / `Ctrl+C` | Quit |

### Input Commands

Press `i` to enter input mode, then type a command:

| Command | Description |
|---------|-------------|
| `/join <group_id>` | Join a chat group |
| `/leave <group_id>` | Leave a chat group |
| `/groups` | Refresh the list of groups |
| `/approve <id> <grant\|deny> [reason]` | Act on an approval request |
| *(any other text)* | Send as a message to the currently selected agent |

Press `Enter` to execute, `Esc` to cancel.

### Message Filtering

1. Press `/` to enter filter mode
2. Type your search query — messages are filtered in real-time by tag, agent name, or payload content
3. The center pane title updates to show match count (e.g. `Messages (5 of 100)`)
4. Press `Enter` to lock the filter, or `Esc` to clear it

### Agent Filtering

1. Press `f` to toggle agent filter mode (only works when left pane is focused)
2. Type a name or capability to narrow the agent list
3. Press `Enter` to apply, `Esc` to clear

### Auto-reconnect

The TUI includes built-in reconnection logic. If the WebSocket connection drops, it automatically retries every 2 seconds until restored. A periodic tick (every 5 seconds) also triggers reconnection attempts and data refreshes.

---

## Makefile Targets

Development workflow targets for building, testing, and running the project.

| Target | Command | Description |
|--------|---------|-------------|
| `make build` | Compiles all 3 binaries | Outputs `bin/bobberd`, `bin/bobber`, `bin/bobber-tui` |
| `make test` | `go test ./backend/... ./cli/... ./tui/...` | Run unit tests across all modules |
| `make test-integration` | `go test -tags=integration -race ./backend/test/integration/ -v` | Run integration tests (requires PostgreSQL) |
| `make test-api` | `go test -tags=integration -race ./backend/test/api/ -v -count=1` | Run API tests |
| `make test-e2e` | `./scripts/e2e-test.sh` | Run end-to-end tests (requires `docker compose up`) |
| `make lint` | `go vet ./backend/... ./cli/... ./tui/...` | Lint all packages |
| `make migrate` | `psql -f migrations/001_initial_schema.sql` | Apply database migrations |
| `make run-backend` | `go run ./backend/cmd/bobberd --config configs/backend.yaml` | Start the backend server locally |
| `make run-tui` | `go run ./tui/cmd/bobber-tui` | Start the TUI client locally |
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

# In another terminal, start the TUI
make run-tui
```
