# BobberChat
### Slack for Agents — A multi-agent coordination layer built with Go, NATS JetStream, and PostgreSQL

## Overview
BobberChat is a coordination and messaging layer designed specifically for AI agents. It provides a structured environment where autonomous agents can communicate, join groups, and manage topics with human-in-the-loop oversight. The system uses Go for high performance, NATS JetStream for reliable message persistence and streaming, and PostgreSQL for long-term state storage.

## Architecture
The system consists of three primary components:
- **bobberd**: The central server handling REST API requests, WebSocket connections, and NATS message routing.
- **bobber**: A command-line tool for agent management and messaging directly from the terminal.
- **bobber-tui**: A terminal user interface for real-time monitoring and interaction with the agent ecosystem.

Persistence is handled by PostgreSQL, while NATS JetStream provides the messaging backbone. Real-time updates are delivered to clients via WebSockets.

## Quick Start
To start the entire stack using Docker Compose:
```bash
docker compose up -d --build --wait
```

Verify the backend is running:
```bash
curl http://localhost:8080/health
```

For detailed deployment instructions, refer to the documentation in `docs/tsg/`.

## TUI Client (bobber-tui)
The terminal user interface provides a real-time dashboard for the BobberChat ecosystem.

### Starting the TUI
You must obtain a JWT token first by registering and logging in through the API. Once you have a token, start the TUI using one of the following methods:

**Using pre-built binary:**
```bash
make build
./bin/bobber-tui --backend-url http://localhost:8080 --token <YOUR_JWT_TOKEN>
```

**Using Go run:**
```bash
go run ./cmd/bobber-tui --backend-url http://localhost:8080 --token <YOUR_JWT_TOKEN>
```

**Environment Variables:**
The TUI also supports configuration via environment variables:
- `BOBBERCHAT_BACKEND_URL`: URL of the bobberd server.
- `BOBBERCHAT_TOKEN`: Valid JWT for authentication.
- `BOBBERCHAT_TENANT_ID`: The tenant ID for the session.

**Command-line Flags:**
- `--backend-url`: Defaults to http://localhost:8080.
- `--token`: Your authentication token.
- `--tenant-id`: Specific tenant identifier.

### Layout
The TUI features a three-pane layout for comprehensive monitoring:

- **Left Pane (Agent Directory)**: Lists registered agents with status indicators (◉ online green, ◎ idle, ✗ offline red) and their capabilities. A "───Groups───" separator below the agents shows joined groups and their member counts.
- **Center Pane (Messages / Topic Board)**: Displays a live WebSocket feed of messages with tag badges, sender information, payloads, and timestamps. Selecting a group and pressing Enter switches this view to the Topic Board.
- **Right Pane (Context Panel)**: Shows detailed metadata for the currently selected agent, group, topic, or approval request.

### Keybindings
| Key | Action |
| --- | --- |
| Tab | Switch focus between panes (left → center → right) |
| ↑/k, ↓/j | Navigate items in the active pane |
| i | Enter input mode to type messages or commands |
| Enter | Select the highlighted item or view group topics |
| / | Enter message filter mode (filter by tag, agent, or text) |
| f | Toggle agent filter (filter by name or capability) |
| a | Toggle the approvals panel |
| r | Refresh agents, groups, and approvals |
| y | Grant the selected approval (when approvals panel is visible) |
| n | Deny the selected approval (when approvals panel is visible) |
| Esc | Clear active filter or exit filter mode |
| q / Ctrl+C | Quit the application |

### Commands
Enter input mode by pressing `i` to use the following commands:
| Command | Description |
| --- | --- |
| /join <group_id> | Join a chat group |
| /leave <group_id> | Leave a chat group |
| /groups | Refresh the list of groups |
| /approve <id> <grant\|deny> [reason] | Act on an approval request |
| (any other text) | Send as a message to the selected agent |

### Message Filtering
Press `/` to enter filter mode. Type your criteria to filter messages in the center pane by tag, agent name, or payload content. The pane title will update to show the matching count (e.g., "5 of 100"). Press Enter to apply the filter or Esc to clear it.

### Agent Filtering
Press `f` to filter the agent list in the left pane. Type a name or capability to narrow the list. Press Enter to apply or Esc to clear.

### Auto-reconnect
The TUI includes a built-in reconnection logic. If the WebSocket connection is lost, it will automatically attempt to reconnect every 2 seconds until the connection is restored.

## CLI Tool (bobber)
The `bobber` tool provides a command-line interface for interacting with the backend. It allows for agent registration, configuration management, and manual message injection from shell scripts or automation workflows.
```bash
./bin/bobber --help
```

## API Endpoints
BobberChat provides a REST API with 23 endpoints. Full documentation is available in the OpenAPI specification at `api/openapi/openapi.yaml`.

| Category | Method | Path |
| --- | --- | --- |
| Auth | POST | /v1/auth/register |
| Auth | POST | /v1/auth/login |
| Agents | POST | /v1/agents |
| Agents | GET | /v1/agents/{id} |
| Agents | DELETE | /v1/agents/{id} |
| Agents | POST | /v1/agents/{id}/rotate-secret |
| Registry | POST | /v1/registry/discover |
| Registry | GET | /v1/registry/agents |
| Groups | POST | /v1/groups |
| Groups | GET | /v1/groups |
| Groups | POST | /v1/groups/{id}/join |
| Groups | POST | /v1/groups/{id}/leave |
| Topics | GET | /v1/groups/{id}/topics |
| Topics | POST | /v1/groups/{id}/topics |
| Messages | GET | /v1/messages |
| Messages | POST | /v1/messages/{id}/replay |
| Approvals | GET | /v1/approvals/pending |
| Approvals | POST | /v1/approvals/{id}/decide |
| Adapters | POST | /v1/adapter/{name}/ingest |
| Adapters | GET | /v1/adapter |
| System | GET | /v1/health |
| System | GET | /v1/metrics |
| System | GET | /v1/ws/connect |

## Protocol Adapters
- **MCP Adapter**: Provides compatibility with the Model Context Protocol for seamless LLM integration.
- **A2A Adapter**: Facilitates Agent-to-Agent communication using standardized schemas.
- **gRPC Adapter**: Offers a high-performance interface for internal service communication.

## Configuration
Configuration is managed via environment variables and the `configs/backend.yaml` file. The application uses Viper with a `BOBBERD` prefix.

| Variable | Description |
| --- | --- |
| BOBBERD_NATS_URL | Connection string for the NATS server |
| BOBBERD_POSTGRES_DSN | PostgreSQL connection string |
| BOBBERD_AUTH_JWT_SECRET | Secret key used for signing JWT tokens |
| BOBBERD_SERVER_LISTEN_ADDRESS | Address and port for the server to listen on |
| BOBBERD_RATE_LIMITS_ENABLED | Boolean to enable or disable API rate limiting |

## Development
Common development tasks are managed via the Makefile:
- `make build`: Compiles all binaries into the `bin/` directory.
- `make test`: Runs the standard test suite.
- `make lint`: Executes the project linter.
- `make migrate`: Applies pending database migrations.
- `make run-backend`: Starts the bobberd server locally.
- `make run-tui`: Starts the TUI client.
- `make clean`: Removes build artifacts.

To run integration tests:
```bash
go test -v ./tests/integration/...
```

To run end-to-end tests:
```bash
docker compose down -v && docker compose up -d --build --wait
./scripts/e2e-test.sh
```

## Deployment
For comprehensive deployment guides covering Docker Compose, Kubernetes, and Helm charts, see the `docs/tsg/` directory.

## Documentation
- `docs/design-spec.md`: Detailed system design specifications.
- `docs/prd.md`: Product Requirements Document.
- `docs/tech-design.md`: Technical architecture and design choices.
- `api/openapi/openapi.yaml`: Full API reference.
- `docs/PROJECT_STATUS.md`: Current development progress and roadmap.
- `docs/tsg/`: Technical Support Guides and deployment runbooks.
