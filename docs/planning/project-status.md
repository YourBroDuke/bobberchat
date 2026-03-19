# BobberChat Project Status & Continuation Guide

> Last updated: 2026-03-19
> Branch: `master`
> Repo: `https://github.com/YourBroDuke/bobberchat.git`

---

## Current State: All Implementation & Documentation Complete

All core modules, protocol adapters, production hardening, CI/CD & deployment, README, and documentation are implemented, compiled, tested, and verified end-to-end.

### Build & Test Verification

```bash
go build ./backend/... ./cli/...    # ✅ Clean (Go workspace)
go vet ./backend/... ./cli/...      # ✅ Clean
go test ./backend/... ./cli/...     # ✅ 15 packages pass (~245+ subtests)

# Docker-based E2E
docker compose up -d
./scripts/e2e-test.sh                    # ✅ 27/27 pass

# Integration tests (requires running PostgreSQL via Docker Compose)
BOBBERCHAT_TEST_DSN="postgres://bobberchat:bobberchat@localhost:5432/bobberchat?sslmode=disable" \
go test -tags=integration ./backend/test/integration/ -v    # ✅ 3/3 pass
```

---

## What's Done

### Documentation (3 docs + CLI reference + OpenAPI spec + README + TSG)

| File | Lines | Description |
|------|-------|-------------|
| `docs/architecture/design-spec.md` | 1,693 | Authoritative design spec — 13 sections + glossary + 4 appendices |
| `docs/planning/prd.md` | 212 | Product requirements document |
| `docs/architecture/tech-design.md` | 721 | Technical design document |
| `api/openapi/openapi.yaml` | ~1,530 | OpenAPI 3.1.0 spec — 31 endpoint paths |
| `README.md` | ~280 | Comprehensive project README |
| `docs/reference/cli-reference.md` | ~770 | Complete CLI reference for bobber and bobberd |
| `docs/operations/deploy-docker-compose.md` | ~120 | Docker Compose deployment guide |
| `docs/operations/deploy-kubernetes.md` | ~130 | Raw Kubernetes manifests deployment guide |
| `docs/operations/deploy-helm.md` | ~170 | Helm chart deployment guide |
| `docs/operations/deploy-local.md` | ~120 | Local development setup guide |
| `docs/operations/troubleshooting.md` | ~200 | Common issues and fixes (Docker, API, K8s, Helm) |
| `docs/operations/manual-testing.md` | ~210 | Step-by-step manual testing walkthrough with curl |

### Core Implementation (8 packages)

| Package | File | Lines | Description |
|---------|------|-------|-------------|
| `backend/internal/protocol` | `envelope.go`, `tags.go`, `version.go` | ~350 | Wire envelope, 7-family tag taxonomy, version negotiation |
| `backend/internal/persistence` | `postgres.go`, `models.go`, `repositories.go` | ~1,250 | 11 repository interfaces with PostgreSQL implementations, including conversations (with last_message tracking), conversation participants, polymorphic connection requests and polymorphic blacklist (user/agent/group pairs) persistence |
| `backend/internal/auth` | `auth.go` | ~503 | Argon2id hashing, JWT (HS256, 1hr TTL), bcrypt for passwords, email verification and resend flows |
| `backend/internal/email` | `email.go`, `azurecs/azurecs.go`, `console/console.go` | ~214 | Provider-agnostic email sender interface with console and Azure Communication Services (ACS) sender implementations. ACS sender uses HMAC-SHA256 signed REST API calls (`/emails:send`) with connection-string auth |
| `backend/internal/registry` | `registry.go` | ~115 | Agent discovery and listing |
| `backend/internal/broker` | `broker.go` | ~232 | NATS JetStream message routing, 3 streams, subject mapping |
| `backend/internal/conversation` | `conversation.go` | ~202 | Conversations (DM & group), membership via conversation_participants, message history, list by type |
| `backend/internal/observability` | `observability.go` | ~110 | Prometheus metrics (incl. rate limit, audit, active WS conn gauges), structured logging |

### Protocol Adapters (3 packages — Design Spec §8)

| Package | Files | Lines | Description |
|---------|-------|-------|-------------|
| `backend/internal/adapter` | `adapter.go` | ~49 | Shared `Adapter` interface, `TransportMeta`, metadata helpers |
| `backend/internal/adapter/mcp` | `mcp.go`, `mcp_test.go` | ~673 | MCP JSON-RPC bridge: `tool/call` → `request.action`, `tool/result` → `response.*`, notifications → `context-provide`. 13 tests |
| `backend/internal/adapter/a2a` | `a2a.go`, `a2a_test.go` | ~994 | A2A protocol bridge: `message/send` → `request.*` (intent inference), `agent/card` → `context-provide`, `task/*` → lifecycle mapping. 20 tests |
| `backend/internal/adapter/grpc` | `grpc.go`, `grpc_test.go` | ~901 | gRPC JSON bridge: unary calls, responses, stream frames with percentage extraction. 22 tests |

All adapters implement the shared `Adapter` interface:
```go
type Adapter interface {
    Name() string
    Protocol() string
    Ingest(ctx context.Context, raw []byte, meta TransportMeta) (*protocol.Envelope, error)
    Emit(ctx context.Context, env *protocol.Envelope) ([]byte, error)
    Validate(raw []byte) error
}
```

### System Metadata Refactor (Tags & Payload → Metadata)

All system-injected data has been moved from `Tag` and `Payload` fields into `Metadata` under underscore-prefixed keys. This ensures `Tag` and `Content` remain purely user-controlled input. The `Payload map[string]any` field was subsequently replaced with `Content string` across the entire codebase.

**Changes across all adapters** (gRPC, MCP, A2A):
- `Ingest()` now writes system data to `Metadata` (e.g. `_tag`, `_action`, `_args`) instead of `Tag`/`Content`
- `Emit()` now reads system data from `Metadata` instead of `Tag`/`Content`
- All adapter tests updated to assert `Metadata` keys

**Envelope changes** (`internal/protocol/envelope.go`):
- 18 `MetaSys*` constants for all system metadata keys
- `EffectiveTag(env)` helper: prefers user-supplied `Tag`, falls back to `Metadata["_tag"]`
- `Validate()` accepts empty `Tag` when `_tag` exists in Metadata
- `Tag` field is now `json:"tag,omitempty"`

**Adapter helpers** (`internal/adapter/adapter.go`):
- `SetSystemMeta(env, key, value)` — write system metadata
- `SystemMeta(env, key)` / `SystemMetaString(env, key)` — read system metadata

**Broker & main server**: Use `protocol.EffectiveTag(env)` for routing, rate limiting, metrics, and audit.

**Replay handler**: Replay keys (`_replayed`, `_original_message_id`, `_replay_reason`, `_tag`) in Metadata.

Server endpoints for adapters:
- `POST /v1/adapter/{name}/ingest` — Auth-protected generic ingest endpoint
- `GET /v1/adapter` — List registered adapters

### Production Hardening (Design Spec §11-§12)

| Package / File | Lines | Description |
|----------------|-------|-------------|
| `backend/internal/ratelimit/ratelimit.go` | ~160 | Token bucket rate limiter with per-agent, per-group, per-tag dimensions. Configurable rates, burst factor, auto-cleanup |
| `backend/internal/ratelimit/ratelimit_test.go` | ~230 | 10 tests: basic limit, burst, refill, agent/group/tag scoping, concurrent, cleanup, disabled |
| `backend/cmd/bobberd/main.go` (additions) | ~140 | `publishAndAudit` method: ownership-based access control (§11.3), rate limiting (§11.2.3). Enhanced graceful shutdown with `activeConns` drain-wait |
| `backend/cmd/bobberd/main_test.go` | ~230 | 8 tests: cross-owner denial, rate limiting (agent/group/tag), disabled limiter, empty caller owner |
| `backend/internal/observability/observability.go` (additions) | ~20 | `RateLimited` counter vec, `ActiveWSConns` gauge |
| `configs/backend.yaml` (additions) | ~10 | `rate_limits` config section with per-agent/group/tag rates and burst factor |

Key implementation details:
- **Ownership-based access control**: `publishAndAudit` verifies message sender ownership and group membership (returns 403 on violation)
- **Rate limiting**: Token bucket per-agent, per-group, per-tag. Configurable via `configs/backend.yaml`. Returns 429 when exceeded
- **Graceful shutdown**: `activeConns sync.WaitGroup` tracks live WebSocket connections; shutdown drains with timeout
- **All 4 publish call sites** (`handleSendMessage`, `handleReplayMessage`, `handleAdapterIngest`, `handleWebSocket`) route through `publishAndAudit`

### Binaries (2 commands — Go Workspace)

| Binary | Source | Lines | Description |
|--------|--------|-------|-------------|
| `bobberd` | `backend/cmd/bobberd/main.go` | ~1,374 | Backend server — 36 REST endpoints + WebSocket + message replay + adapter ingest + production hardening |
| `bobber` | `cli/cmd/bobber/main.go` | ~880 | CLI tool — account, agent (create/use/rotate-secret/delete), session, connection, messaging, conversation, blacklist (add/remove/list), and group management commands. `agent use` fetches info, rotates secret, and saves credentials. Tests in `main_test.go` |

### SDK

| File | Description |
|------|-------------|
| `backend/pkg/sdk/types.go` | SDK type definitions |
| `backend/pkg/sdk/helpers.go` | Message construction helpers |
| `backend/pkg/sdk/client.go` | WebSocket client with auto-reconnect |
| `backend/pkg/sdk/config.go` | Configuration loader |

### Tests (15 packages, ~245+ subtests)

| Test File | Subtests | What's Tested |
|-----------|----------|---------------|
| `backend/internal/protocol/envelope_test.go` | 15 | Envelope marshaling, validation (incl. metadata-tag fallback), ID generation |
| `backend/internal/protocol/tags_test.go` | 28 | Tag parsing, validation, family classification |
| `backend/internal/protocol/version_test.go` | 21 | Version negotiation, compatibility checks |
| `backend/internal/auth/auth_test.go` | 10 | Argon2id hash/verify, JWT sign/validate, bcrypt |
| `backend/internal/broker/broker_test.go` | 8 | Subject construction, routing logic |
| `backend/internal/registry/registry_test.go` | — | Input validation |
| `backend/internal/conversation/conversation_test.go` | — | Input validation |
| `backend/internal/adapter/mcp/mcp_test.go` | 13 | MCP ingest/emit, validation, error cases |
| `backend/internal/adapter/a2a/a2a_test.go` | 20 | A2A ingest/emit, intent inference, error cases |
| `backend/internal/adapter/grpc/grpc_test.go` | 22 | gRPC ingest/emit, stream frames, error cases |
| `backend/internal/ratelimit/ratelimit_test.go` | 10 | Token bucket limiting, burst, refill, scoping, concurrent, cleanup |
| `backend/cmd/bobberd/main_test.go` | 8 | Cross-owner denial, rate limiting, audit trail, disabled limiter |
| `backend/pkg/sdk/helpers_test.go` | 4 | Message helper functions |
| `cli/cmd/bobber/main_test.go` | — | CLI unit tests: account register/login, agent create/use/rotate-secret/delete, login/whoami/logout, ls, connect/inbox/accept/reject/blacklist add/remove/list, info, send, poll, group create/leave/invite, config/flag precedence |
| `backend/test/integration/persistence_test.go` | 3 | User, Agent, Group CRUD (build-tagged `//go:build integration`) |
| `scripts/e2e-test.sh` | 27 | API checks for auth, agents, groups, adapters, metrics, and negative auth/error paths |

### Infrastructure

| File | Description |
|------|-------------|
| `Dockerfile` | Multi-stage build (`golang:latest` → `alpine:3.19`), workspace-aware, copies migrations |
| `docker-compose.yml` | 4 services: `nats`, `postgres`, `init-db` (migration), `bobberd` with health checks |
| `migrations/001_initial_schema.sql` | Full schema — 7 tables, 4 enum types, 8 indexes, default partition |
| `migrations/002_email_verification.sql` | Adds `users.email_verified`, verification token columns, and partial token index |
| `migrations/003_connections_blacklist.sql` | Adds `connection_requests` and `blacklist_entries` tables, enum, and indexes |
| `migrations/004_remove_agent_status.sql` | Removes `agent_status` enum type, `status` column, and associated index from agents table |
| `migrations/005_remove_agent_version_heartbeat.sql` | Removes `version`, `connected_at`, `last_heartbeat` columns from agents table |
| `migrations/016_agent_connections.sql` | Renames `from_user_id`/`to_user_id` → `from_agent_id`/`to_agent_id` in `connection_requests`, re-points FKs to `agents(agent_id)` |
| `migrations/018_connection_request_polymorphic.sql` | Refactors `connection_requests` to polymorphic model — adds `sender_id`, `from_kind`, `to_kind` (entity_type enum), renames columns to `from_id`/`to_id` |
| `configs/backend.yaml` | Default backend configuration |
| `Makefile` | Build, test, lint, migrate, run targets |
| `scripts/e2e-test.sh` | 27-test curl-based API e2e test script |
| `backend/test/integration/persistence_test.go` | 3 integration tests (build-tagged `//go:build integration`) |
---

## What's Left To Do

### ~~Priority 1: Production Hardening~~ ✅ COMPLETE

- [x] Rate limiting middleware (design spec §11.2) — Token bucket per-agent/group/tag in `backend/internal/ratelimit/`
- [x] Ownership-based access control (design spec §11.3) — `publishAndAudit` verifies message sender ownership
- [x] Graceful shutdown with drain (design spec §12.5) — `activeConns` WaitGroup with timeout
- [x] WebSocket ping/pong keepalive — already existed in `handleWebSocket`
- [x] Agent heartbeat timeout detection — already existed (`missedPongs` counter, `heartbeatTTL`)
- [ ] NATS JetStream consumer recovery on reconnect — deferred (NATS client handles basic reconnect)

### Priority 3: CI/CD & Deployment ✅ COMPLETE

- [x] GitHub Actions CI workflow (lint, build, unit tests, integration tests, E2E tests, Docker build)
- [x] GitHub Actions release workflow (Docker image publish to GHCR, release binaries)
- [x] Kubernetes raw manifests (namespace, configmap, secrets, nats, postgres, bobberd, migration Job)
- [x] Helm chart (deployment, nats, postgres, secrets, configmap, migration hook, ingress)
- [x] Database migration runner (psql-based via Makefile, K8s Job, and Helm hook)

CI/CD files added:
- `.github/workflows/ci.yml` — 7-job CI pipeline (lint, build, test, integration, E2E, Docker build, Docker push)
- `.github/workflows/release.yml` — Release pipeline (multi-platform Docker push to GHCR, cross-compiled binaries)
- `deploy/k8s/` — 7 raw Kubernetes manifests (namespace, configmap, secrets, nats, postgres, bobberd+migration, cert-manager-issuers)
- `deploy/helm/bobberchat/` — Full Helm chart with 8 templates + helpers + configurable values

### Priority 4: Email Verification & Azure Communication Services ✅ COMPLETE

- [x] `email.Sender` interface for provider-agnostic email sending (`backend/internal/email/email.go`)
- [x] Console/dev sender for local development (`backend/internal/email/console/console.go`)
- [x] Azure Communication Services sender with HMAC-SHA256 REST API auth (`backend/internal/email/azurecs/azurecs.go`)
- [x] Database migration for `email_verified`, `verification_token`, `verification_token_expires_at` columns (`migrations/002_email_verification.sql`)
- [x] Registration flow generates verification token and sends email
- [x] Login blocks unverified users (`"email not verified"`)
- [x] `POST /v1/auth/verify-email` — verifies token and marks user as verified
- [x] `POST /v1/auth/resend-verification` — generates new token and re-sends email
- [x] Azure resources provisioned (staging + production ACS instances with email-enabled managed domains)
- [x] End-to-end tested: register → email sent via ACS → verify token → login succeeds

Azure resources:
- **Staging**: `acs-bobberchat-staging` in `rg-bobberchat-staging` with managed domain
- **Production**: `acs-bobberchat-production` in `rg-bobberchat-production` with managed domain

---

## Bugs Fixed During E2E Testing

1. **Persistence models missing JSON tags**: All structs in `backend/internal/persistence/models.go` had no `json` struct tags, causing API responses to use Go's default capitalized field names (e.g., `ID` instead of `id`). Added proper `json` tags with `json:"-"` for sensitive fields (`PasswordHash`, `APISecretHash`).

2. **NATS Docker healthcheck**: NATS 2.10 image doesn't include `wget` binary. Changed healthcheck from `wget -qO- http://localhost:8222/healthz` to `CMD /nats-server --help` and added `-m 8222` flag to enable monitoring endpoint.

3. **Integration test setup**: `setupDB()` failed when running against an already-migrated database (from `init-db` container). Added schema drop before migration re-apply.

---

## Key Technical Details

### Module & Dependencies

```
Go Workspace (go.work) with 2 independent modules:

  backend/go.mod — github.com/bobberchat/bobberchat/backend
    nats.go v1.49.0         — NATS JetStream messaging
    pgx/v5 v5.8.0           — PostgreSQL driver
    jwt/v5 v5.3.1           — JWT tokens
    uuid v1.6.0             — UUID generation
    gorilla/websocket v1.5.3 — WebSocket server
    prometheus v1.23.2      — Metrics
    cobra v1.10.2           — CLI framework (unused, can remove)
    viper v1.21.0           — Configuration
    zerolog v1.34.0         — Structured logging
    crypto v0.49.0          — Argon2id, bcrypt

  cli/go.mod — github.com/bobberchat/bobberchat/cli
    cobra v1.10.2           — CLI framework
    viper v1.21.0           — Configuration
    uuid v1.6.0             — UUID generation

Go version: 1.25.0 (go.mod)
```

### Configuration

Backend config: `configs/backend.yaml`
- Viper prefix: `BOBBERD`, key replacer `.` → `_`
- Example: `BOBBERD_NATS_URL` → `nats.url`, `BOBBERD_POSTGRES_DSN` → `postgres.dsn`
- JWT secret: `auth.jwt_secret` (must change from default `change-me`)

### Database

- PostgreSQL 15+
- 6 tables: `users`, `agents`, `chat_groups`, `conversations`, `conversation_participants`, `messages`
- 2 enum types: `participant_type`, `conversation_type`
- `conversations` table unifies DMs and groups; DMs identified by canonical `(id_low, id_high)` pair (generic UUIDs, no FK constraint); group conversations linked via `group_id` (UUID FK → chat_groups, ON DELETE SET NULL) for accelerated group lookups
- `conversation_participants` replaces `chat_group_members` and handles both DM and group membership with `muted`, `last_read_message_id` fields
- `messages` table uses `conversation_id` FK (replaced `to_id`), time-based partitioning by `timestamp` (monthly ranges), `participant_kind` column (reuses `participant_type` enum) to distinguish user vs agent messages
- `conversations` table includes `last_message_id` (UUID FK → messages, ON DELETE SET NULL) and `last_message_at` (TIMESTAMPTZ) for efficient conversation ordering
- `pgMessageRepository.Save` atomically inserts the message and updates `conversations.last_message_id/last_message_at` in a single transaction
- Migration: `migrations/001_initial_schema.sql` through `migrations/022_conversation_group_id.sql`

### NATS JetStream Streams

| Stream | Subject Pattern | Retention |
|--------|----------------|-----------|
| `BOBBER_MSG` | `bobberchat.msg.>`, `bobberchat.group.>` | 30 days |
| `BOBBER_SYSTEM` | `bobberchat.system.>` | 24 hours |

Subject pattern: `bobberchat.msg.{to_id}` for direct messages, `bobberchat.group.{group_id}` for groups

### REST API Endpoints (30 total)

```
Auth:       POST /v1/auth/register, /v1/auth/login, /v1/auth/verify-email, /v1/auth/resend-verification, GET /v1/auth/me
Agents:     POST /v1/agents, GET/DELETE /v1/agents/:id, POST /v1/agents/:id/rotate-secret
Registry:   GET /v1/registry/agents, POST /v1/registry/discover
Groups:     POST/GET /v1/groups, POST /v1/groups/:id/join, /v1/groups/:id/leave
Messages:   GET /v1/messages/poll, POST /v1/messages/send, POST /v1/messages/:id/replay
Connections: POST /v1/connections/request, GET /v1/connections/inbox, POST /v1/connections/:id/accept, POST /v1/connections/:id/reject
Blacklist:  GET /v1/blacklist, POST /v1/blacklist, DELETE /v1/blacklist/:id
Adapters:   POST /v1/adapter/{name}/ingest, GET /v1/adapter
WebSocket:  GET /v1/ws/connect
System:     GET /v1/health, /v1/metrics
```

### Wire Envelope (7 fields)

```json
{
  "id": "uuid",
  "from": "uuid",
  "to": "uuid",
  "tag": "(optional) user-supplied tag",
  "content": "",
  "metadata": { "_tag": "request.action", "_action": "...", "...": "..." },
  "timestamp": "RFC3339"
}
```

**Field semantics**: `tag` and `content` are purely user-controlled. All system-injected data (adapter-derived routing tags, action names, error codes, replay keys) lives in `metadata` under underscore-prefixed keys (e.g. `_tag`, `_action`, `_result`). When `tag` is empty, `protocol.EffectiveTag()` falls back to `metadata._tag`.

### 7 Tag Families

`request.*`, `response.*`, `context-provide`, `no-response`, `progress.*`, `error.*`, `system.*`

---

## Quick Start for New Session

```
# Prompt to paste into a new AI session:

I'm continuing work on the BobberChat project. Read docs/planning/project-status.md for full context.

The project is a "Slack for Agents" — a multi-agent coordination layer built with Go, NATS JetStream, and PostgreSQL.

The codebase uses a Go workspace (go.work) with 2 independent modules: backend/, cli/. Each has its own go.mod.

All planned work is COMPLETE: core implementation, protocol adapters, production hardening, CI/CD & deployment, and CLI test coverage. All code compiles, unit tests pass (~245+ subtests across 15 packages), E2E tests pass (27/27), and integration tests pass (3/3).

Follow the existing codebase patterns. Run `go build ./backend/... ./cli/...` and `go test ./backend/... ./cli/...` to verify.
For E2E: `docker compose up -d && ./scripts/e2e-test.sh`
```

---

## File Tree (Key Files Only)

```
bobberchat/
├── go.work                                   # Go workspace (backend, cli)
├── .github/workflows/
│   ├── ci.yml                            # CI pipeline (lint, build, test, integration, E2E, Docker build, Docker push)
│   ├── deploy-staging.yml                # Staging deployment pipeline
│   ├── deploy-production.yml             # Production deployment pipeline
│   └── release.yml                       # Release pipeline (Docker push, binaries)
├── api/openapi/openapi.yaml              # OpenAPI 3.1.0 spec
├── backend/
│   ├── go.mod                            # Backend module: github.com/bobberchat/bobberchat/backend
│   ├── go.sum
│   ├── cmd/
│   │   └── bobberd/
│   │       ├── main.go                   # Backend server (~1150 lines)
│   │       └── main_test.go              # publishAndAudit tests (8 tests)
│   ├── internal/
│   │   ├── adapter/
│   │   │   ├── adapter.go                # Shared Adapter interface (49 lines)
│   │   │   ├── mcp/mcp.go               # MCP adapter (311 lines)
│   │   │   ├── mcp/mcp_test.go          # MCP tests (362 lines, 13 tests)
│   │   │   ├── a2a/a2a.go               # A2A adapter (494 lines)
│   │   │   ├── a2a/a2a_test.go          # A2A tests (~20 tests)
│   │   │   ├── grpc/grpc.go             # gRPC adapter (401 lines)
│   │   │   └── grpc/grpc_test.go        # gRPC tests (~22 tests)
│   │   ├── auth/auth.go                  # Auth (Argon2id + JWT + email verification)
│   │   ├── email/
│   │   │   ├── email.go                  # Email sender interface
│   │   │   ├── azurecs/azurecs.go        # Azure Communication Services sender (HMAC-SHA256 REST API)
│   │   │   └── console/console.go        # Console sender for local/dev
│   │   ├── broker/broker.go              # NATS JetStream routing
│   │   ├── conversation/conversation.go  # Groups, history
│   │   ├── observability/observability.go# Metrics, logging (~110 lines)
│   │   ├── persistence/                  # PostgreSQL repositories
│   │   │   ├── models.go                 # Models with JSON struct tags
│   │   │   ├── postgres.go
│   │   │   └── repositories.go
│   │   ├── protocol/                     # Wire protocol
│   │   │   ├── envelope.go
│   │   │   ├── tags.go
│   │   │   └── version.go
│   │   ├── ratelimit/
│   │   │   ├── ratelimit.go              # Token bucket rate limiter (~160 lines)
│   │   │   └── ratelimit_test.go         # 10 rate limiter tests
│   │   └── registry/registry.go          # Agent discovery
│   ├── pkg/sdk/                          # Go SDK
│   │   ├── client.go
│   │   ├── config.go
│   │   ├── helpers.go
│   │   └── types.go
│   └── test/integration/
│       └── persistence_test.go           # Build-tagged DB tests
├── cli/
│   ├── go.mod                            # CLI module: github.com/bobberchat/bobberchat/cli
│   ├── go.sum
│   └── cmd/bobber/
│       ├── main.go                       # CLI tool (634 lines)
│       └── main_test.go                  # CLI tests (refactored to match new command structure)
├── configs/backend.yaml                  # Default config
├── deploy/
│   ├── k8s/                              # Raw Kubernetes manifests
│   │   ├── namespace.yml
│   │   ├── configmap.yml
│   │   ├── secrets.yml
│   │   ├── nats.yml
│   │   ├── postgres.yml
│   │   ├── bobberd.yml                   # Backend + migration Job + migrations ConfigMap
│   │   └── cert-manager-issuers.yaml     # Let's Encrypt ClusterIssuer definitions
│   └── helm/bobberchat/                  # Helm chart
│       ├── Chart.yaml
│       ├── values.yaml
│       └── templates/
│           ├── _helpers.tpl
│           ├── deployment.yaml
│           ├── nats.yaml
│           ├── postgres.yaml
│           ├── secrets.yaml
│           ├── configmap.yaml
│           ├── migration.yaml
│           └── ingress.yaml
├── deploy/terraform/                     # Terraform infrastructure
│   ├── bootstrap/                        # One-time backend state setup
│   ├── environments/                     # Staging & production configs
│   └── modules/                          # Reusable infra modules (network, aks, database, dns)
├── docker-compose.yml                    # 4 services with health checks
├── Dockerfile                            # Multi-stage build (workspace-aware)
├── docs/
│   ├── architecture/
│   │   ├── design-spec.md            # Authoritative spec (1,693 lines)
│   │   └── tech-design.md            # Technical design
│   ├── operations/
│   │   ├── ci-cd.md                  # CI/CD pipeline documentation
│   │   ├── deploy-azure.md           # Azure AKS deployment guide
│   │   ├── deploy-docker-compose.md  # Docker Compose deployment
│   │   ├── deploy-kubernetes.md      # Raw K8s manifests deployment
│   │   ├── deploy-helm.md            # Helm chart deployment
│   │   ├── deploy-local.md           # Local dev setup
│   │   ├── troubleshooting.md        # Common issues & fixes
│   │   └── manual-testing.md         # Hands-on curl walkthrough
│   ├── planning/
│   │   ├── prd.md                    # Product requirements
│   │   └── project-status.md         # ← THIS FILE
│   └── reference/
│       └── cli-reference.md          # Complete CLI reference (bobber, bobberd, Makefile)
├── migrations/001_initial_schema.sql  # Full DB schema
├── migrations/002_email_verification.sql # User email verification columns/index
├── migrations/003_connections_blacklist.sql # Connection request and blacklist persistence
├── migrations/004_remove_agent_status.sql # Removes agent_status enum/column/index
├── migrations/005_remove_agent_version_heartbeat.sql # Removes version, connected_at, last_heartbeat from agents
├── migrations/006_remove_topics.sql # Removes topics table, topic_id column, and topic_status enum
├── migrations/007_blacklist.sql     # Adds agent blacklist table
├── migrations/008_conversations.sql # Adds conversations, conversation_participants; migrates chat_group_members; messages.to_id→conversation_id
├── migrations/009_rename_dm_ids.sql # Renames agent_id_low/agent_id_high → id_low/id_high in conversations
├── migrations/013_conversation_last_message.sql # Adds last_message_id, last_message_at to conversations with backfill
├── migrations/016_agent_connections.sql # Renames connection_requests FKs from user to agent
├── migrations/017_add_message_participant_kind.sql # Adds participant_kind column to messages table
├── migrations/018_connection_request_polymorphic.sql # Polymorphic connection_requests: sender_id, from/to kind (agent/group)
├── migrations/019_blacklist_polymorphic.sql # Polymorphic blacklist_entries: from/to id+kind (user/agent/group)
├── migrations/021_conversation_last_message_at_default.sql # Defaults last_message_at to now(), backfills NULLs
├── migrations/022_conversation_group_id.sql # Adds group_id FK to conversations for accelerated group lookups
├── scripts/
│   ├── e2e-test.sh                  # 27-test API e2e test
│   └── smoke-test.sh                # Quick deployment smoke test
├── Makefile
└── README.md
```
