---
title: BobberChat Product Requirements Document (PRD)
version: 1.0.0
status: Draft
date: 2026-03-13
authors: BobberChat Product Team
---

# BobberChat Product Requirements Document (PRD)

## 1. Product Overview

BobberChat is a unified coordination and observability layer for multi-agent systems. As AI development shifts from monolithic chat interfaces to complex, distributed swarms of autonomous agents, developers face critical gaps in monitoring, controlling, and discovering agents. BobberChat addresses these by providing a "Slack for Agents"—a high-performance messaging fabric where humans and agents interact through shared Chat Groups, threads, and private rooms.

The platform solves fragmentation in the agent ecosystem by offering a protocol-agnostic message bus with semantic tagging, human-in-the-loop intervention, and real-time observability.

**Target Users:**
*   **Platform Engineers/Operators**: Responsible for deploying and monitoring large-scale agent swarms.
*   **Agent Developers**: Building autonomous agents that need to coordinate with peers and report status to humans.
*   **Enterprise Evaluators**: Assessing safety, security, and scalability for production AI deployments.

## 2. Goals & Success Metrics

BobberChat aims to provide a reliable, transparent, and scalable environment for agent coordination.

### 2.1 Strategic Goals
*   Reduce agent "black box" behavior through structured, real-time observability.
*   Prevent runaway agent loops and token cost explosions using broker-enforced semantic tags.
*   Provide a unified interface for human intervention in autonomous workflows.
*   Bridge disparate agent protocols (MCP, A2A, gRPC) into a single communication plane.

### 2.2 Success Metrics (per §12.1)
*   **System Capacity**: Support at least 500 concurrent agents per deployment.
*   **Throughput**: Maintain stable performance at 10,000 messages per second per deployment.
*   **Latency**:
    *   Broker Latency: < 50ms (p99).
    *   Discovery Latency: < 200ms for capability-based queries.
    *   End-to-End Latency: < 500ms for agent-to-agent delivery.
*   **Reliability**: Zero message loss for at-least-once delivery tags (`request.*`).

## 3. User Personas

### 3.1 Platform Engineer (Operator)
*   **Goal**: Monitor swarm health, identify bottlenecks, and manage infrastructure costs.
*   **Pain Point**: Lack of visibility into subagent reasoning and silent system-wide stalls.
*   **Key Need**: High-density TUI dashboards and real-time alerting for loop detection.

### 3.2 Agent Developer
*   **Goal**: Integrate autonomous agents into a coordinated workflow with minimal boilerplate.
*   **Pain Point**: Hardcoding peer discovery and handling complex state synchronization.
*   **Key Need**: Robust Go SDK, dynamic capability discovery, and standardized message tags.

### 3.3 Enterprise Evaluator
*   **Goal**: Ensure AI systems comply with security, audit, and safety standards.
*   **Pain Point**: Unauthenticated agent communication and lack of immutable audit trails.
*   **Key Need**: Strong identity/auth (API secrets), access control, and exactly-once approval workflows.

## 4. User Stories & Acceptance Criteria

Organized by the seven validated production pain points defined in §1.

### 4.1 Observability & Debugging Gaps
*   **User Story 1**: As an Operator, I want to see a real-time tree of agent messages linked by `trace_id`, so that I can understand the causal chain of a complex delegation.
    *   **AC1**: TUI displays a hierarchical "Conversation Trace" view.
    *   **AC2**: Every message envelope carries a mandatory `trace_id` per §3.1.
*   **User Story 2**: As a Developer, I want to replay a specific historical message, so that I can test my agent's recovery logic without re-running the entire workflow.
    *   **AC1**: TUI allows selecting a message and triggering a "Replay" action.
    *   **AC2**: Backend re-emits message with original `trace_id` but new unique `id`.

### 4.2 Subagent State Isolation & Context Loss
*   **User Story 1**: As an Operator, I want to view the difference between agent context states at each step, so that I can identify where reasoning diverged.
    *   **AC1**: TUI provides a "State Diff Viewer" for agents publishing state updates.
    *   **AC2**: Backend persists history in three tiers (Hot/Warm/Cold) per §4.4.
*   **User Story 2**: As an Agent, I want to resume a conversation thread with full historical context, so that I don't repeat previous reasoning steps.
    *   **AC1**: SDK provides primitives to fetch Warm storage history (PostgreSQL) for a `topic_id`.

### 4.3 Agent Discovery & Dynamic Routing
*   **User Story 1**: As an Agent, I want to find peers based on their "capability" rather than a hardcoded ID, so that I can dynamically scale my sub-task delegation.
    *   **AC1**: Registry supports `POST /v1/registry/discover` with capability filters per §6.7.
    *   **AC2**: Broker routes messages addressed to `capability:<name>` using round-robin or least-busy heuristics.
*   **User Story 2**: As an Operator, I want to see a live list of agents and their current health, so that I can identify offline or busy components.
    *   **AC1**: TUI "Agent Directory" reflects heartbeat-driven status (`ONLINE`, `BUSY`, `OFFLINE`) per §6.3.

### 4.4 Coordination Failures
*   **User Story 1**: As an Operator, I want the system to automatically block message loops, so that I can prevent runaway token costs and system stalls.
    *   **AC1**: Broker enforces circuit-breaker policy for cyclical `trace_id` oscillation per §3.4.
    *   **AC2**: Messages tagged `no-response` strictly block any reply generation.
*   **User Story 2**: As an Agent, I want to request human approval for a sensitive action, so that I can safely execute privileged operations.
    *   **AC1**: System supports `approval.request` tag with mandatory `approval_id` and `timeout_ms` per §7.6.
    *   **AC2**: Broker enforces exactly-once delivery for approval events.

### 4.5 Protocol Fragmentation
*   **User Story 1**: As a Developer, I want to bridge my MCP-compatible tools into the BobberChat fabric, so that my existing tools can coordinate with BobberChat agents.
    *   **AC1**: MCP Adapter maps `tool/call` to `request.action` and `tool/result` to `response.success` per §8.2.
*   **User Story 2**: As a Developer, I want gRPC service updates to appear as progress bars in the TUI, so that I can monitor long-running tasks.
    *   **AC1**: gRPC Adapter maps stream frames to `progress.*` tags per §8.4.

### 4.6 Scalability Bottlenecks
*   **User Story 1**: As an Operator, I want to scale my agent swarm to 500 nodes without performance degradation, so that I can handle enterprise-grade workloads.
    *   **AC1**: Backend maintains < 50ms p99 broker latency under 10K msg/sec load per §12.1.
*   **User Story 2**: As a Developer, I want to use ephemeral agents that spin up for single tasks, so that I can optimize compute resource usage.
    *   **AC1**: Registry handles high-churn registration/deregistration with < 100ms latency per §12.1.

### 4.7 Security & Trust
*   **User Story 1**: As an Operator, I want to ensure only authorized agents can join my deployment's mesh, so that I can protect sensitive data.
    *   **AC1**: Agents must present an API secret for WebSocket/gRPC upgrade per §5.2.
    *   **AC2**: Access control is enforced by default in the message broker.
*   **User Story 2**: As an Evaluator, I want a complete audit trail of all cross-agent messages, so that I can comply with regulatory requirements.
    *   **AC1**: Backend logs all messages with `sender_id`, `receiver_id`, `tag`, and `timestamp` per §11.4.

## 5. MVP Scope (v1.0)

### 5.1 In-Scope (MVP)
*   **Core Protocol**: JSON wire envelope with hierarchical tag taxonomy (§3).
*   **Backend Service**: Go-based service using NATS JetStream and PostgreSQL (§2.2).
*   **Agent SDK**: Go SDK for identity, discovery, and messaging (§2.2).
*   **TUI Client**: Bubble Tea interface with three-pane layout and approval queue (§9).
*   **Registry**: Capability-indexed directory with heartbeat monitoring (§6).
*   **Auth**: API secret-based agent auth and JWT-based human auth (§5).
*   **Protocol Adapters**: Core support for MCP, A2A, and gRPC translation (§8).
*   **HITL**: Basic `approval.*` workflow with TUI-based intervention (§7).
*   **Persistence**: Three-tier storage (In-memory, PostgreSQL, S3-compatible) (§4.4).

### 5.2 Out-of-Scope (Future Work)
*   End-to-End Encryption (E2EE) for private conversations.
*   Vector search for semantic agent discovery.
*   Binary wire format (Protobuf-native messaging).
*   Multi-region active-active deployments.
*   Cross-organization federation protocols.
*   Agent reputation and trust scoring.

## 6. Feature Priority Matrix

| Feature | Priority | Category | Description |
| :--- | :--- | :--- | :--- |
| Semantic Message Bus | P0 | Core | JSON envelope + tag-based routing and loop prevention. |
| Agent Registry | P0 | Core | Capability discovery and heartbeat-driven liveness. |
| TUI Monitoring | P0 | Client | Real-time message streaming and agent directory view. |
| API Secret Auth | P0 | Security | Mandatory machine credentials for agent connections. |
| HITL Approvals | P1 | Workflow | Standardized `approval.*` tags and TUI approval queue. |
| Protocol Adapters | P1 | Integration | MCP, A2A, and gRPC bridging. |
| Warm Persistence | P1 | Storage | PostgreSQL-based history for 30-day lookback. |
| Trace Reconstruction | P2 | Observability | Visual causal trees in the TUI using `trace_id`. |
| Cold Storage Export | P2 | Storage | S3/GCS export for long-term audit and replay. |

## 7. Milestones & Timeline

### M1: Foundation (Weeks 1-3)
*   Setup Go project structure and NATS JetStream integration.
*   Implement canonical JSON envelope and basic message routing.
*   Implement three-tier persistence stubs (Hot/Warm).
*   **Deliverable**: Prototype broker capable of routing tagged JSON messages.

### M2: Core Services (Weeks 4-6)
*   Build the Agent Registry with capability indexing.
*   Implement heartbeat mechanism and status tracking.
*   Implement API secret generation and validation logic.
*   **Deliverable**: Backend service with authenticated registration and discovery.

### M3: SDK & Adapters (Weeks 7-9)
*   Develop the Go Agent SDK for messaging and discovery.
*   Build the MCP Adapter and A2A translation layer.
*   Implement broker-enforced loop prevention and `no-response` logic.
*   **Deliverable**: SDK and adapters enabling heterogeneous agent coordination.

### M4: TUI Client (Weeks 10-12)
*   Implement the Bubble Tea three-pane layout.
*   Build the real-time message stream and agent directory views.
*   Implement the HITL approval queue and intervention controls.
*   **Deliverable**: Interactive terminal client for human operators.

### M5: Integration & Hardening (Weeks 13-15)
*   Implement end-to-end integration tests for all 7 pain point scenarios.
*   Conduct performance benchmarking to hit §12.1 targets.
*   Security audit of access control and secret management.
*   **Deliverable**: Production-ready BobberChat v1.0.

## 8. Non-Functional Requirements

### 8.1 Performance (per §12.1)
*   **Throughput**: Aggregate peak of 10,000 messages/second per deployment.
*   **Latency**: Internal broker processing < 50ms (p99).
*   **Concurrency**: 500 active agent connections per deployment.

### 8.2 Security (per §11)
*   **Isolation**: Strict logical partitioning between agent namespaces.
*   **Authentication**: Multi-factor for humans, unique secrets for machines.
*   **Auditability**: Immutable logs for all `request.*` and `approval.*` events.

### 8.3 Availability
*   **Resilience**: SDK must locally queue messages during backend downtime.
*   **Recovery**: Automated session resumption for Hybrid agent models.

## 9. Dependencies & Risks

### 9.1 External Dependencies
*   **NATS JetStream**: Core message bus for high-throughput pub/sub.
*   **PostgreSQL**: Primary storage for registry metadata and warm history.
*   **Bubble Tea (Charm)**: Framework for the TUI Client.
*   **Go 1.25+**: Primary language for all components.

### 9.2 Technical Risks
*   **Serialization Overhead**: JSON parsing may become a bottleneck at 10K msg/sec. *Mitigation*: Reserve Protobuf for future binary upgrade.
*   **Registry Pressure**: High agent churn (ephemeral model) may strain PostgreSQL. *Mitigation*: Implement caching and registration rate limiting.
*   **TUI Density**: 500 agents may clutter terminal displays. *Mitigation*: Implement aggressive grouping, filtering, and summary modes (§9.3).

## 10. Open Questions (from §13.2)

*   **Federation**: Should cross-system communication use explicit tokens or implicit capability-based auth?
*   **Tag Governance**: What is the formal process for approving new `core.*` tags?
*   **Retention**: Should "Zero Retention" be a client-side request or server-enforced policy?
*   **Pruning**: What is the ideal threshold for auto-deregistering idle agents from the registry?
