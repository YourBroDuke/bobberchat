# BobberChat Design Specification

## TL;DR

> **Quick Summary**: Design a complete architecture specification for BobberChat — an open-source TUI IM product that solves multi-agent cross-node communication pain points through a custom tagged-message protocol, agent discovery registry, and human-in-the-loop approval workflows. The deliverable is a single Markdown design document (~30-40 pages) covering protocol design, system architecture, conversation model, identity/auth, TUI concepts, and observability.
>
> **Deliverables**:
> - Single design specification document (`.md`) covering all architectural aspects
> - Includes: architecture diagrams (Mermaid/ASCII), protocol spec, message tag taxonomy, API sketches, TUI wireframes, security considerations
>
> **Estimated Effort**: Medium (15 writing tasks + 4 verification tasks)
> **Parallel Execution**: YES - 7 waves (Waves 2-3 have 5 parallel tasks each)
> **Critical Path**: Skeleton → Protocol/Tags (§3) + Identity (§5) → Dependent sections → Future Work/Appendices → Final Pass → Verification

---

## Context

### Original Request
User wants to build a TUI IM product to solve the key pain points that multi-agent cross-node communication faces. Requested research into pain points and a prototype design specification.

### Interview Summary
**Key Discussions**:
- **Product Name**: BobberChat
- **Architecture**: Three-component split — K8s cloud backend (broker, registry, persistence), Agent SDK/CLI (primary integration), TUI client (single binary for human observation/control)
- **Protocol**: Custom protocol with extensible hierarchical message tags (request, context-provide, no-response, progress, error + custom) to prevent agent feedback loop storms
- **Conversation Model**: Private chat (1:1), Chat Groups (named channels), Topics (auto-threaded tasks within groups)
- **Identity/Auth**: SaaS model — email registration → create agents (limited free, unlimited premium) → API secret per agent
- **Scale**: 50-500+ agents, enterprise-grade, multi-datacenter capable
- **Tech Stack**: Go + Bubble Tea v2 (TUI), K8s backend
- **Licensing**: Open source (MIT/Apache)
- **Deliverable**: Architecture overview / design specification document (NOT code)

**Research Findings**:
- 7 validated pain points: observability gaps, context/state loss, agent discovery, coordination failures, protocol fragmentation, scalability bottlenecks, security/trust
- No existing product solves cross-node agent message visualization + protocol translation
- Message tags map to FIPA ACL performatives but modernized for LLM agents
- A2A v1.0 (Linux Foundation) and MCP (Anthropic) are dominant but have critical gaps
- Developer pain: agent looping (28%), token cost explosions (22%), silent failures (19%)
- NATS JetStream recommended for message broker (290K+ msgs/sec, K8s-native)

### Metis Review
**Identified Gaps (addressed)**:
- Protocol versioning & evolution strategy → included in §3
- Offline/disconnected agent behavior & delivery guarantees → included in §3
- Message size limits & context window awareness → included in §3
- Conflict resolution for multi-agent disagreements → included in §7
- Observability data model (OpenTelemetry traces, metrics) → included in §10
- Data residency & compliance considerations → included in §11
- TUI scaling at 500 agents (information density) → included in §9
- Agent lifecycle models (persistent, ephemeral, hybrid) → included in §5
- Tag enforcement: broker-level vs advisory → included in §3
- Cross-tenant agent communication → included in §11

---

## Work Objectives

### Core Objective
Produce a comprehensive architecture overview document for BobberChat that is sufficient to begin implementation, covering system architecture, custom protocol with message tags, conversation model, identity/auth, agent discovery, approval workflows, protocol adapters, TUI design, observability, and security.

### Concrete Deliverables
- Single Markdown file: the BobberChat Design Specification document
- 13 content sections + appendices (see structure below)
- Mermaid/ASCII diagrams embedded inline
- Glossary of domain terms
- Open questions clearly marked

### Definition of Done
- [ ] All 13 sections have content (no stubs/placeholders)
- [ ] All core message tags (~15) defined with hierarchy, fields, delivery semantics, example JSON
- [ ] At least 5 Mermaid/ASCII diagrams (architecture, message flows, auth flow, protocol state machine, TUI wireframe)
- [ ] Every section has: problem statement, design decision, rationale
- [ ] All 7 pain points traceable to specific design decisions
- [ ] Document renders correctly as GitHub-flavored Markdown
- [ ] No unresolved forward references

### Must Have
- Custom message tag taxonomy with extensible hierarchy
- Three-component architecture (backend, SDK, TUI)
- Conversation model (private, groups, topics)
- SaaS identity/auth model with API secrets
- Agent discovery and registry design
- Approval workflow design
- Protocol adapter strategy (MCP/A2A bridging)
- Security considerations
- Observability data model

### Must NOT Have (Guardrails)
- Runnable code — only pseudocode and illustrative JSON
- Full SDK API surface design (limit to capability requirements + 3-5 examples)
- Billing/payment/pricing system design
- Admin dashboard design
- Deployment infrastructure (no Helm charts, Terraform, CI/CD)
- Exhaustive message tag catalog beyond ~15 core tags + extension mechanism
- Exact library versions or dependency trees
- UI details like key bindings or color schemes (conceptual wireframes only)

---

## Verification Strategy (MANDATORY)

> **ZERO HUMAN INTERVENTION** — ALL verification is agent-executed. No exceptions.

### Test Decision
- **Infrastructure exists**: N/A (design document, not code)
- **Automated tests**: N/A
- **Framework**: N/A

### QA Policy
Every task MUST include agent-executed QA scenarios that verify:
1. Section completeness (all required subsections present)
2. Internal consistency (cross-references valid, terms match glossary)
3. Markdown rendering (no broken Mermaid, valid GFM)
4. Pain point traceability (each section maps to validated problems)

Evidence saved to `.sisyphus/evidence/task-{N}-{scenario-slug}.{ext}`.

- **Document sections**: Use Bash (grep/wc) — Verify section headers, word count, diagram presence
- **Markdown quality**: Use Bash (markdownlint or grep for common issues) — Verify rendering
- **Consistency**: Use Bash (grep) — Cross-reference tag names, component names, term usage

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Foundation — skeleton only, blocks everything):
└── Task 1: Document skeleton (ToC, metadata, glossary stubs) [quick]

Wave 2 (Independent sections — all depend only on Task 1):
├── Task 2: §1 Executive Summary & Problem Statement [writing]
├── Task 3: §2 System Architecture Overview [writing]
├── Task 4: §3 Custom Protocol & Message Tag Taxonomy [deep]
├── Task 5: §5 Identity, Authentication & Agent Lifecycle [deep]
└── Task 6: §4 Conversation Model [writing]

Wave 3 (Dependent sections — build on protocol + identity + architecture):
├── Task 7: §6 Agent Discovery & Registry [writing]
├── Task 8: §7 Approval Workflows & Coordination Primitives [writing]
├── Task 9: §8 Protocol Adapters (MCP/A2A/gRPC) [deep]
├── Task 10: §9 TUI Client Design & Layout [visual-engineering]
└── Task 11: §10 Observability & Debugging [writing]

Wave 4 (Cross-cutting sections — depend on Wave 2-3):
├── Task 12: §11 Security Considerations [writing]
└── Task 13: §12 Scalability & Performance [writing]

Wave 5 (Synthesis — depends on ALL prior tasks):
└── Task 14: §13 Future Work, Open Questions & Appendices [writing]

Wave 6 (Final pass — depends on Tasks 1-14):
└── Task 15: Final consistency pass, cross-references, glossary completion [deep]

Wave FINAL (Verification — after Task 15):
├── Task F1: Plan compliance audit (subagent_type="oracle")
├── Task F2: Document quality review (unspecified-high)
├── Task F3: Pain point traceability verification (deep)
└── Task F4: Scope fidelity check (deep)

Critical Path: Task 1 → Task 4 → Task 8 → Task 14 → Task 15 → F1-F4
Parallel Speedup: ~55% faster than sequential
Max Concurrent: 5 (Waves 2 and 3)
```

### Dependency Matrix

| Task | Depends On | Blocks | Wave |
|------|-----------|--------|------|
| 1 (Skeleton) | — | 2,3,4,5,6 | 1 |
| 2 (§1 Exec Summary) | 1 | 14 | 2 |
| 3 (§2 Architecture) | 1 | 7,8,9,10,11,13 | 2 |
| 4 (§3 Protocol/Tags) | 1 | 7,8,9,11,12 | 2 |
| 5 (§5 Identity/Auth) | 1 | 7,8,12 | 2 |
| 6 (§4 Conversation) | 1 | 7,8,10 | 2 |
| 7 (§6 Discovery) | 3,4,5 | 14 | 3 |
| 8 (§7 Approval) | 4,5,6 | 14 | 3 |
| 9 (§8 Adapters) | 3,4 | 14 | 3 |
| 10 (§9 TUI) | 3,6 | 14 | 3 |
| 11 (§10 Observability) | 3,4 | 14 | 3 |
| 12 (§11 Security) | 4,5 | 14 | 4 |
| 13 (§12 Scalability) | 3 | 14 | 4 |
| 14 (§13 Future/Appendix) | 2-13 | 15 | 5 |
| 15 (Final Pass) | 1-14 | F1-F4 | 6 |
| F1-F4 (Verification) | 15 | — | FINAL |

### Agent Dispatch Summary

- **Wave 1**: **1** — T1 → `quick`
- **Wave 2**: **5** — T2 → `writing`, T3 → `writing`, T4 → `deep`, T5 → `deep`, T6 → `writing`
- **Wave 3**: **5** — T7 → `writing`, T8 → `writing`, T9 → `deep`, T10 → `visual-engineering`, T11 → `writing`
- **Wave 4**: **2** — T12 → `writing`, T13 → `writing`
- **Wave 5**: **1** — T14 → `writing`
- **Wave 6**: **1** — T15 → `deep`
- **FINAL**: **4** — F1 → `subagent_type="oracle"`, F2 → `unspecified-high`, F3 → `deep`, F4 → `deep`

---

## TODOs

- [x] 1. Document Skeleton — ToC, Metadata, Glossary Stubs

  **What to do**:
  - Create the design spec Markdown file with document metadata header (title: "BobberChat Design Specification", version: 0.1.0, status: Draft, date, authors)
  - Write a "How to Read This Document" section explaining the audience (protocol implementor, SDK developer, TUI contributor, enterprise evaluator)
  - Create the full Table of Contents with 13 numbered sections + appendices
  - Create a Glossary section with stub entries for all domain terms: agent, node, topic, tag, channel, group, SDK, TUI, message broker, registry, approval workflow, protocol adapter, delivery guarantee, agent card, context window
  - Add a "Notation & Conventions" subsection explaining: MUST/SHOULD/MAY (RFC 2119), OPEN QUESTION markers, diagram notation

  **Must NOT do**:
  - Write content for any section beyond headers/stubs
  - Include any implementation-specific details

  **Recommended Agent Profile**:
  - **Category**: `quick`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (sole task in Wave 1)
  - **Parallel Group**: Wave 1
  - **Blocks**: Tasks 2, 3, 4, 5, 6
  - **Blocked By**: None

  **References**:
  - Research summary in `.sisyphus/drafts/tui-im-agent-mesh.md` — all confirmed requirements, decisions, and research findings
  - RFC 2119 notation: https://datatracker.ietf.org/doc/html/rfc2119

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Document skeleton has all 13 section headers
    Tool: Bash (grep)
    Steps:
      1. grep -c "^## §" docs/design-spec.md
      2. Assert count >= 13
      3. Verify each section header matches the plan (§1 through §13)
    Expected Result: All 13 section headers present
    Evidence: .sisyphus/evidence/task-1-section-headers.txt

  Scenario: Glossary has all required terms
    Tool: Bash (grep)
    Steps:
      1. grep -c "^\*\*" the glossary section
      2. Assert at least 15 term entries present
    Expected Result: All domain terms have stub entries
    Evidence: .sisyphus/evidence/task-1-glossary-terms.txt
  ```

  **Commit**: YES
  - Message: `docs(design): scaffold BobberChat design spec skeleton`
  - Files: `docs/design-spec.md`

- [x] 2. §1 Executive Summary & Problem Statement

  **What to do**:
  - Write a 1-2 paragraph executive summary capturing BobberChat's value proposition: "The coordination layer multi-agent systems are missing"
  - Document the 7 validated pain points as the problem statement, with evidence citations (AgentRx, LangGraph issues, developer surveys)
  - Include a "Why BobberChat?" subsection explaining how the product addresses each pain point at a high level
  - Include a "Competitive Landscape" subsection showing: no existing tool solves cross-node agent message visualization + protocol translation (reference SwarmWatch, Agent View, AgentDbg and their limitations)
  - Add a market context paragraph: 72% of AI projects use multi-agent systems in 2026, 40% fail in production due to coordination problems

  **Must NOT do**:
  - Go into technical detail (that's for later sections)
  - Include any protocol specifications

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 3, 4, 5, 6)
  - **Blocks**: Task 14
  - **Blocked By**: Task 1

  **References**:
  - `.sisyphus/drafts/tui-im-agent-mesh.md` — "Top 7 Pain Points Identified" section with evidence
  - `.sisyphus/drafts/tui-im-agent-mesh.md` — "Existing TUI Tools for Agents" section (competitive landscape)
  - Research findings from librarian agents: AgentRx (+23.6%), LangGraph #573/#1698/#1923, developer survey (28% looping, 22% cost, 19% silent failures)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 7 pain points documented
    Tool: Bash (grep)
    Steps:
      1. Count pain point entries in §1
      2. Verify each has an evidence citation
    Expected Result: 7 pain points with citations
    Evidence: .sisyphus/evidence/task-2-pain-points.txt

  Scenario: Section has required subsections
    Tool: Bash (grep)
    Steps:
      1. Verify "Executive Summary", "Problem Statement", "Why BobberChat", "Competitive Landscape" subsections exist
    Expected Result: All 4 subsections present
    Evidence: .sisyphus/evidence/task-2-subsections.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §1 - Executive Summary & Problem Statement`
  - Files: `docs/design-spec.md`

- [x] 3. §2 System Architecture Overview

  **What to do**:
  - Design and document the three-component architecture with a Mermaid diagram:
    1. **Backend Service** (K8s cloud): Message broker, agent registry, persistence layer, protocol adapters, approval engine
    2. **Agent SDK/CLI**: Go SDK + CLI tool for agents to connect, send tagged messages, discover peers, handle approvals
    3. **TUI Client** (single binary): Human interface for observability, approval workflows, agent management
  - Define the communication topology: SDK → Backend (WebSocket/gRPC), TUI → Backend (WebSocket), Backend internal (NATS JetStream)
  - Include a Mermaid architecture diagram showing all components and their connections
  - Define component responsibilities clearly (what each component does and does NOT do)
  - Include a "Technology Recommendations" subsection (not normative): Go, NATS JetStream, PostgreSQL, Bubble Tea v2
  - Address data flow patterns: how a message travels from Agent A → Backend → Agent B, and how TUI observes it

  **Must NOT do**:
  - Specify deployment infrastructure (no Helm, Terraform)
  - Design the full backend microservice decomposition
  - Include exact library versions

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 4, 5, 6)
  - **Blocks**: Tasks 7, 8, 9, 10, 11, 13
  - **Blocked By**: Task 1

  **References**:
  - `.sisyphus/drafts/tui-im-agent-mesh.md` — "Architecture Clarification" section (3-component split)
  - Research: NATS JetStream (290K+ msgs/sec, K8s-native, 50MB footprint)
  - Research: Bubble Tea v2 architecture (declarative views, concurrent WebSocket via p.Send())
  - k9s architecture as reference for TUI monitoring patterns

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Architecture diagram present and valid Mermaid
    Tool: Bash (grep)
    Steps:
      1. Find Mermaid code block in §2
      2. Verify it contains: "Backend", "SDK", "TUI" nodes
      3. Verify connection arrows between components
    Expected Result: Valid Mermaid diagram with all 3 components
    Evidence: .sisyphus/evidence/task-3-architecture-diagram.txt

  Scenario: All 3 components have responsibility definitions
    Tool: Bash (grep)
    Steps:
      1. Verify "Backend Service", "Agent SDK/CLI", "TUI Client" subsections exist
      2. Each has "Responsibilities" and "Does NOT" clauses
    Expected Result: 3 component definitions with clear boundaries
    Evidence: .sisyphus/evidence/task-3-components.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §2 - System Architecture Overview`
  - Files: `docs/design-spec.md`

- [x] 4. §3 Custom Protocol Design & Message Tag Taxonomy

  **What to do**:
  - **This is the most architecturally consequential section.** Design the BobberChat Wire Protocol:
    - Message envelope format (JSON): `{ id, from, to, tag, payload, metadata, timestamp, trace_id }`
    - Required and optional fields
    - 3-5 example message payloads for different tag types
  - **Message Tag Taxonomy** — define ~15 core tags with extensible hierarchy:
    - `request` — expects a response. Sub-tags: `request.data`, `request.approval`, `request.action`
    - `response` — reply to a request. Sub-tags: `response.success`, `response.error`, `response.partial`
    - `context-provide` — informational, no response expected
    - `no-response` — explicitly suppress reply (loop prevention)
    - `progress` — status update, not actionable. Sub-tags: `progress.percentage`, `progress.milestone`
    - `error` — error report. Sub-tags: `error.fatal`, `error.recoverable`
    - `approval` — approval workflow messages. Sub-tags: `approval.request`, `approval.granted`, `approval.denied`
    - `system` — system-level messages. Sub-tags: `system.join`, `system.leave`, `system.heartbeat`
    - Custom tags: namespace convention (`org.example.custom-tag`)
  - For each core tag: name, parent, description, required payload fields, delivery semantics (exactly-once/at-least-once/best-effort), whether broker enforces behavior, example JSON
  - **Loop Prevention Mechanics**: How `no-response` and `context-provide` tags are enforced at broker level (broker drops responses to `no-response` messages). Include circuit breaker pattern.
  - **Delivery Guarantees** per tag category: `request.*` = at-least-once with timeout, `progress.*` = best-effort, `approval.*` = exactly-once
  - **Protocol Versioning**: Semantic versioning on wire format, tag namespace versioning, version negotiation on connection handshake
  - **Message Size Limits**: Max payload size per tier (free: 64KB, premium: 1MB), `context-budget` metadata field for agents to declare remaining token capacity
  - Include a protocol state machine diagram (Mermaid) showing message tag transitions (e.g., request → response | timeout → error)

  **Must NOT do**:
  - Enumerate every possible custom tag (define extension mechanism only)
  - Specify binary wire format (JSON is the wire format for architecture overview level)
  - Design the full serialization library

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 3, 5, 6)
  - **Blocks**: Tasks 7, 8, 9, 11, 12
  - **Blocked By**: Task 1

  **References**:
  - `.sisyphus/drafts/tui-im-agent-mesh.md` — "Message Tag System" and "Novel Feature: Message Tags for Loop Prevention" sections
  - Research: FIPA ACL performatives (REQUEST, INFORM, PROPOSE, COMMIT) — modernize for LLM agents
  - Research: A2A JSON-RPC message format — study for interop considerations
  - Research: Agent looping (28% of issues) — circuit breaker patterns from production systems
  - Research: Token cost explosions (22%) — context-budget tag addresses this

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Core tag taxonomy has ~15 tags defined
    Tool: Bash (grep)
    Steps:
      1. Count tag definition entries in the taxonomy table
      2. Verify each has: name, parent, description, delivery semantics, enforcement
    Expected Result: 12-18 core tags fully defined
    Evidence: .sisyphus/evidence/task-4-tag-taxonomy.txt

  Scenario: Message envelope format documented with examples
    Tool: Bash (grep)
    Steps:
      1. Find JSON example blocks in §3
      2. Count at least 3 different message examples
      3. Verify envelope has: id, from, to, tag, payload, metadata, timestamp
    Expected Result: 3+ JSON examples with complete envelope fields
    Evidence: .sisyphus/evidence/task-4-message-examples.txt

  Scenario: Protocol state machine diagram present
    Tool: Bash (grep)
    Steps:
      1. Find Mermaid stateDiagram block in §3
      2. Verify it shows request→response and request→timeout→error transitions
    Expected Result: Valid state machine diagram
    Evidence: .sisyphus/evidence/task-4-state-machine.txt

  Scenario: Loop prevention mechanism documented
    Tool: Bash (grep)
    Steps:
      1. Search for "loop prevention" or "circuit breaker" content
      2. Verify broker-level enforcement of no-response tag is described
    Expected Result: Concrete loop prevention mechanism described
    Evidence: .sisyphus/evidence/task-4-loop-prevention.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §3 - Custom Protocol & Message Tag Taxonomy`
  - Files: `docs/design-spec.md`

- [x] 5. §5 Identity, Authentication & Agent Lifecycle

  **What to do**:
  - **Identity Model**: 
    - Human users: email registration → user account → can create agents
    - Agents: created under user account, limited count on free tier, unlimited on premium
    - Each agent gets: unique agent_id (UUID), display name, API secret (generated on creation)
    - Agent metadata: name, description, capabilities list, version, owner (user_id), created_at
  - **Authentication Flow**:
    - Agent → Backend: API secret in Authorization header (Bearer token) on WebSocket upgrade
    - TUI → Backend: JWT from user login (email + password → JWT)
    - Include an auth flow Mermaid diagram showing: agent registration → API secret → connection → authenticated session
  - **Agent Lifecycle Models** (address Metis gap):
    - **Persistent agents**: Long-running, maintain WebSocket connection, heartbeat-based health
    - **Ephemeral agents**: Short-lived, connect → execute → disconnect. Registration persists, connection is temporary
    - **Hybrid agents**: Persistent registration with intermittent connections (reconnect with session resumption)
    - Define lifecycle state machine: REGISTERED → CONNECTING → ONLINE → BUSY → IDLE → OFFLINE → DEREGISTERED
  - **API Secret Management**: Generation, rotation, revocation. One agent = one active secret (rotation generates new, invalidates old after grace period)
  - **Agent Profiles / Agent Cards**: Metadata structure agents publish on registration (capabilities, supported tags, version, endpoints) — similar to A2A Agent Cards but BobberChat-native

  **Must NOT do**:
  - Design billing/payment tiers (just reference "free tier" and "premium tier" as concepts)
  - Specify password hashing algorithms or JWT implementation details
  - Design user management admin UI

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 3, 4, 6)
  - **Blocks**: Tasks 7, 8, 12
  - **Blocked By**: Task 1

  **References**:
  - `.sisyphus/drafts/tui-im-agent-mesh.md` — "Identity & Auth Model" section
  - Research: A2A Agent Cards format — study for interop
  - Research: Agent lifecycle models (persistent, ephemeral, hybrid) — Metis identified gap
  - Research: SaaS API secret patterns

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 3 agent lifecycle models documented
    Tool: Bash (grep)
    Steps:
      1. Search for "persistent", "ephemeral", "hybrid" agent types in §5
      2. Verify each has description and lifecycle state transitions
    Expected Result: 3 lifecycle models with state machines
    Evidence: .sisyphus/evidence/task-5-lifecycle-models.txt

  Scenario: Auth flow diagram present
    Tool: Bash (grep)
    Steps:
      1. Find Mermaid sequence diagram in §5
      2. Verify it shows agent registration → API secret → connection flow
    Expected Result: Valid auth flow diagram
    Evidence: .sisyphus/evidence/task-5-auth-flow.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §5 - Identity, Authentication & Agent Lifecycle`
  - Files: `docs/design-spec.md`

- [x] 6. §4 Conversation Model (Private Chat, Groups, Topics)

  **What to do**:
  - **Private Chat (1:1)**: Direct messages between any two participants (agent↔agent, human↔agent, human↔human). Delivery semantics: at-least-once. Persistent history.
  - **Chat Groups**: Named channels where multiple participants join/leave. 
    - Properties: name, description, members list, visibility (public/private), creator, created_at
    - Membership: agents and humans can join/leave. Invite-only or open.
    - Message broadcasting: sent to all online members, queued for offline members (with configurable retention)
  - **Topics**: Auto-created threaded conversations within groups for specific tasks/workflows.
    - Created when an agent sends a `request.*` tag with a new topic subject
    - Sub-threads allowed within topics (for subtask decomposition)
    - Topic lifecycle: OPEN → IN_PROGRESS → RESOLVED → ARCHIVED
    - Topics can reference parent topics (hierarchical task decomposition)
  - **Message History & Persistence**: Three-tier model:
    - Hot (in-memory/Redis): last 3 hours, instant retrieval
    - Warm (PostgreSQL): last 30 days, query-based retrieval
    - Cold (object storage): 90+ days, archive retrieval
  - Include a Mermaid diagram showing the relationship between Private Chat, Groups, Topics, and Sub-threads
  - Define message ordering guarantees: per-group causal ordering (within a group, messages respect happens-before), no cross-group ordering guarantee

  **Must NOT do**:
  - Design the storage layer implementation (just define the model)
  - Specify exact Redis/Postgres schemas
  - Design moderation or content filtering

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Tasks 2, 3, 4, 5)
  - **Blocks**: Tasks 7, 8, 10
  - **Blocked By**: Task 1

  **References**:
  - `.sisyphus/drafts/tui-im-agent-mesh.md` — "Conversation Model" section
  - Research: Matrix protocol room model (proven at scale, federated)
  - Research: Slack channel/thread model (UX patterns)
  - Research: Three-tier persistence (Redis hot → Postgres warm → S3 cold)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 3 conversation types documented
    Tool: Bash (grep)
    Steps:
      1. Verify "Private Chat", "Chat Groups", "Topics" subsections exist in §4
      2. Each has properties, membership rules, and message delivery semantics
    Expected Result: 3 conversation types fully described
    Evidence: .sisyphus/evidence/task-6-conversation-types.txt

  Scenario: Topic lifecycle defined
    Tool: Bash (grep)
    Steps:
      1. Search for topic state machine (OPEN → IN_PROGRESS → RESOLVED → ARCHIVED)
      2. Verify sub-thread support documented
    Expected Result: Topic lifecycle with states documented
    Evidence: .sisyphus/evidence/task-6-topic-lifecycle.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §4 - Conversation Model`
  - Files: `docs/design-spec.md`

- [x] 7. §6 Agent Discovery & Registry

  **What to do**:
  - **Registry Design**: Centralized registry in the backend that agents register with on connection
    - Registration data: agent_id, capabilities (list of skills/actions), supported message tags, version, status, connected_at, last_heartbeat
    - Query API: search by capability (semantic search), by tag support, by status, by owner
  - **Discovery Protocol**: 
    - On connect: agent publishes its Agent Card (profile with capabilities)
    - Agents can query: "find agents that support `request.data` tag and have `sql-analysis` capability"
    - Results include: agent_id, name, capabilities, status (online/offline/busy), latency estimate
  - **Health Monitoring**:
    - Heartbeat-based (configurable interval, default 30s)
    - Status states: ONLINE, BUSY, IDLE, OFFLINE, DEGRADED
    - Auto-deregister after N missed heartbeats (configurable)
  - **Capability-Based Routing**: When Agent A sends a `request.data` message without a specific recipient, the broker can route to the best available agent matching the required capability (load-balanced)
  - Address Metis edge case: **Ephemeral agent churn** — rapid register/deregister. Rate limit registration, cache agent profiles beyond connection lifetime.
  - Include a Mermaid sequence diagram showing: agent registration → capability advertisement → discovery query → capability-based routing

  **Must NOT do**:
  - Design vector embedding infrastructure for semantic search (mention it as a capability, don't design it)
  - Implement the full registry API (sketch the key endpoints only)
  - Design a reputation/trust scoring system (mention as Future Work)

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 8, 9, 10, 11)
  - **Blocks**: Task 15
  - **Blocked By**: Tasks 3, 4, 5

  **References**:
  - Research: A2A Agent Cards (`.well-known/agent.json`) — study format for interop
  - Research: No existing tool has capability-based agent discovery
  - Research: K8s DNS + etcd for service discovery patterns
  - Research: NATS micro framework for automatic agent registration
  - Metis: Ephemeral agent lifecycle churn — rate limiting needed

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Discovery query API sketched
    Tool: Bash (grep)
    Steps:
      1. Find API sketch in §6 (endpoint, input, output)
      2. Verify capability-based search is supported
    Expected Result: Discovery API with capability search
    Evidence: .sisyphus/evidence/task-7-discovery-api.txt

  Scenario: Sequence diagram present
    Tool: Bash (grep)
    Steps:
      1. Find Mermaid sequenceDiagram in §6
      2. Verify registration → advertisement → query → routing flow
    Expected Result: Valid discovery flow diagram
    Evidence: .sisyphus/evidence/task-7-discovery-diagram.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §6 - Agent Discovery & Registry`
  - Files: `docs/design-spec.md`

- [x] 8. §7 Approval Workflows & Coordination Primitives

  **What to do**:
  - **Approval Workflow**: 
    - Agent sends `approval.request` message with: action description, justification, urgency level, timeout
    - Backend routes to designated approver (human or supervising agent)
    - TUI shows approval request with context, approve/deny buttons
    - Approver responds with `approval.granted` or `approval.denied` (with reason)
    - Timeout behavior: configurable (auto-deny, auto-approve, escalate to next approver)
  - **Escalation Patterns**:
    - Agent → primary approver → secondary approver → human (fallback chain)
    - Escalation triggers: timeout, confidence threshold, cost threshold
  - **Coordination Primitives** (address Metis gap on conflict resolution):
    - **Priority-based resolution**: Higher-priority agent's decision wins
    - **Voting**: N agents vote, majority/unanimous wins
    - **Designated arbiter**: Specific agent/human has tiebreaker authority
    - **Escalation-to-human**: When agents can't agree, escalate to human via TUI
  - **Anti-patterns addressed**:
    - Token cost budgeting: `request.action` can include `max_cost` field, agents refuse actions exceeding budget
    - Infinite retry prevention: Circuit breaker pattern — after N failed retries, auto-escalate
  - Include at least 3 scenario flowcharts (Mermaid): happy path approval, timeout escalation, rejection with reason
  - Define the `approval.*` tag sub-hierarchy with complete field definitions

  **Must NOT do**:
  - Design the approval UI in detail (that's §9 TUI section)
  - Implement voting algorithms
  - Design audit log storage

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 9, 10, 11)
  - **Blocks**: Task 15
  - **Blocked By**: Tasks 4, 5, 6

  **References**:
  - Research: MCP has `elicitation/request` (pull-based) — BobberChat needs push-based approval notifications
  - Research: No standard for human-in-the-loop coordination in agent protocols
  - Metis: Conflict resolution primitives for multi-agent disagreements
  - Research: Circuit breaker patterns from production (28% agent looping)
  - Research: Token cost explosions (22%) — cost budgeting primitives

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: 3 approval workflow scenarios documented
    Tool: Bash (grep)
    Steps:
      1. Count Mermaid flowchart/sequence diagrams in §7
      2. Verify: happy path, timeout/escalation, rejection scenarios
    Expected Result: At least 3 scenario diagrams
    Evidence: .sisyphus/evidence/task-8-approval-scenarios.txt

  Scenario: Conflict resolution primitives defined
    Tool: Bash (grep)
    Steps:
      1. Search for "priority", "voting", "arbiter", "escalation" in §7
      2. Verify each has description and use case
    Expected Result: 4 conflict resolution primitives documented
    Evidence: .sisyphus/evidence/task-8-conflict-resolution.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §7 - Approval Workflows & Coordination`
  - Files: `docs/design-spec.md`

- [x] 9. §8 Protocol Adapters (MCP/A2A/gRPC Bridging)

  **What to do**:
  - **Adapter Architecture**: Define the adapter interface contract — what goes in (external protocol message), what comes out (BobberChat tagged message)
  - **MCP Adapter**:
    - Maps MCP `tool/call` → BobberChat `request.action` tag
    - Maps MCP `tool/result` → BobberChat `response.success` tag
    - Maps MCP notifications → BobberChat `context-provide` tag
    - Limitations: MCP has no agent identity — adapter assigns synthetic agent_id for MCP servers
  - **A2A Adapter**:
    - Maps A2A `message/send` → BobberChat `request.*` tags (inferred from content/skill)
    - Maps A2A Agent Cards → BobberChat Agent Profiles
    - Maps A2A tasks → BobberChat Topics
    - Bi-directional: BobberChat agents can appear as A2A agents to external systems
  - **gRPC Adapter**:
    - Maps gRPC service calls → BobberChat `request.action` tags
    - Protobuf messages serialized into BobberChat JSON payload
    - Streaming gRPC → BobberChat `progress.*` tags
  - **Tag Auto-Mapping Rules**: How inbound protocol messages are automatically tagged (e.g., HTTP POST → `request`, Server-Sent Event → `progress`)
  - **Adapter Lifecycle**: Adapters run as backend service plugins. Register on startup, advertise bridged capabilities in the registry.
  - Include a mapping table showing protocol ↔ BobberChat tag translations for each adapter

  **Must NOT do**:
  - Implement adapter code
  - Handle every edge case in every protocol version
  - Design adapter SDK (just define the interface)

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 8, 10, 11)
  - **Blocks**: Task 15
  - **Blocked By**: Tasks 3, 4

  **References**:
  - Research: MCP JSON-RPC message format — `tool/call`, `tool/result`, notifications
  - Research: A2A Agent Cards, `message/send`, task lifecycle
  - Research: A2A v1.0 RC under Linux Foundation — latest spec evolution
  - Research: No existing product unifies MCP + A2A + custom protocols
  - Research: LiteLLM has partial A2A-MCP bridge (incomplete)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: All 3 protocol adapters have mapping tables
    Tool: Bash (grep)
    Steps:
      1. Find MCP, A2A, gRPC adapter subsections in §8
      2. Verify each has a protocol ↔ tag mapping table
    Expected Result: 3 mapping tables present
    Evidence: .sisyphus/evidence/task-9-adapter-mappings.txt

  Scenario: Adapter interface contract defined
    Tool: Bash (grep)
    Steps:
      1. Search for adapter interface definition (input/output contract)
      2. Verify it's protocol-agnostic (works for any adapter)
    Expected Result: Generic adapter interface documented
    Evidence: .sisyphus/evidence/task-9-adapter-interface.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §8 - Protocol Adapters (MCP/A2A/gRPC)`
  - Files: `docs/design-spec.md`

- [x] 10. §9 TUI Client Design & Layout

  **What to do**:
  - **Layout Concept**: Three-pane layout (conceptual wireframe, ASCII art):
    - Left pane: Agent/Group list with status indicators (◉ online, ◎ idle, ✗ offline)
    - Center pane: Message view (conversation feed with tagged messages, color-coded by tag type)
    - Right pane: Context panel (agent details, topic state, approval actions)
  - **Key Views**:
    - **Conversation View**: Threaded message display with tag badges, timestamp, sender identity
    - **Agent Directory**: Live list of registered agents with capabilities, status, health metrics
    - **Approval Queue**: Pending approval requests with context, approve/deny actions
    - **Topic Board**: Active topics with status (OPEN/IN_PROGRESS/RESOLVED), assignees, progress
    - **Observability Dashboard**: Message throughput, active connections, error rates (summary)
  - **Information Density at Scale** (address Metis gap):
    - Agent grouping (by owner, capability, status)
    - Filter/search across all conversations
    - Summary/aggregation mode at 100+ agents (group status, not individual)
    - Notification priority levels (CRITICAL → shows immediately, INFO → badge only, DEBUG → hidden by default)
    - Focus mode: track specific agents/topics, suppress everything else
  - **Interaction Model**: Vim-style keybindings (conceptual, not exhaustive), command palette with `:`, tab-based view switching
  - Include ASCII wireframe of the primary three-pane layout
  - Address responsive design: how layout adapts to narrow terminals (<120 cols → compact mode, hide right pane)

  **Must NOT do**:
  - Specify exact key bindings, colors, or themes
  - Design the full widget library
  - Include Bubble Tea component architecture (implementation detail)

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 8, 9, 11)
  - **Blocks**: Task 15
  - **Blocked By**: Tasks 3, 6

  **References**:
  - Research: k9s (gold standard monitoring TUI — drill-down, filtering, Vim keys)
  - Research: aerc (three-pane email — message threading, async IMAP)
  - Research: gomuks (Matrix TUI — room list, E2E encryption UI)
  - Research: Mastui (multi-column dashboard — persistence, image rendering)
  - Research: Agent View (session manager — status indicators, notifications)
  - Metis: TUI scaling at 500 agents — agent grouping, filter, summary mode, notification priority

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: ASCII wireframe of primary layout present
    Tool: Bash (grep)
    Steps:
      1. Find ASCII art or code block wireframe in §9
      2. Verify it shows 3-pane layout (left, center, right)
    Expected Result: Visual wireframe present
    Evidence: .sisyphus/evidence/task-10-wireframe.txt

  Scenario: All key views documented
    Tool: Bash (grep)
    Steps:
      1. Search for: "Conversation View", "Agent Directory", "Approval Queue", "Topic Board"
      2. Verify each has description of content and interaction model
    Expected Result: 4+ key views described
    Evidence: .sisyphus/evidence/task-10-views.txt

  Scenario: Scale handling documented
    Tool: Bash (grep)
    Steps:
      1. Search for grouping, filtering, summary mode, notification priority
      2. Verify strategy for 100+ agents described
    Expected Result: Scale handling strategy present
    Evidence: .sisyphus/evidence/task-10-scale-handling.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §9 - TUI Client Design & Layout`
  - Files: `docs/design-spec.md`

- [x] 11. §10 Observability & Debugging

  **What to do**:
  - **Observability Data Model** (address Metis gap):
    - Trace propagation: every message carries `trace_id` (UUID) and `parent_span_id` for causal chains
    - Span naming convention: `agent:{agent_id}:{tag}` (e.g., `agent:abc123:request.data`)
    - OpenTelemetry compatibility: BobberChat traces can export to OTLP endpoints (Jaeger, Grafana Tempo)
  - **Key Metrics** (defined with names and semantics):
    - `bobberchat.messages.sent` — counter, per-agent, per-tag
    - `bobberchat.messages.latency_ms` — histogram, request→response latency
    - `bobberchat.agents.online` — gauge, currently connected agents
    - `bobberchat.topics.active` — gauge, open topics
    - `bobberchat.approvals.pending` — gauge, waiting approvals
    - `bobberchat.errors.count` — counter, per-agent, per-error-type
  - **Debugging Features**:
    - Message replay: re-send any historical message to reproduce agent behavior
    - Conversation trace: follow a trace_id through all agents it touched
    - State diff viewer: show what changed in a Topic at each message
    - Dependency graph: visualize which agents are blocked waiting on others (from pending `request` tags with no `response`)
  - **Structured Logging**: All agent messages stored with searchable metadata (agent_id, tag, trace_id, group_id, topic_id, timestamp)
  - **Alerting**: Define alert conditions (e.g., "Agent X has > 10 unanswered requests for > 5 minutes" → surface in TUI as CRITICAL notification)

  **Must NOT do**:
  - Design the full alerting engine
  - Specify Grafana dashboard layouts
  - Implement OpenTelemetry exporters

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 3 (with Tasks 7, 8, 9, 10)
  - **Blocks**: Task 15
  - **Blocked By**: Tasks 3, 4

  **References**:
  - Research: Microsoft AgentRx (+23.6% improvement with structured observability)
  - Research: OpenTelemetry trace/span model
  - Research: "You can't debug what you can't see" — #1 pain point
  - Research: Developer pain: silent failures (19%), agent looping (28%)
  - Metis: Observability data model — trace propagation, span naming, metric definitions

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Metrics defined with names and semantics
    Tool: Bash (grep)
    Steps:
      1. Count metric definitions (bobberchat.* names)
      2. Verify each has: name, type (counter/gauge/histogram), description
    Expected Result: 6+ metrics fully defined
    Evidence: .sisyphus/evidence/task-11-metrics.txt

  Scenario: Trace propagation documented
    Tool: Bash (grep)
    Steps:
      1. Search for trace_id, parent_span_id, OpenTelemetry
      2. Verify span naming convention documented
    Expected Result: Trace model documented
    Evidence: .sisyphus/evidence/task-11-trace-model.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §10 - Observability & Debugging`
  - Files: `docs/design-spec.md`

- [x] 12. §11 Security Considerations

  **What to do**:
  - **Threat Model**: Define attack vectors specific to multi-agent systems:
    - Agent impersonation (fake agent_id)
    - Message injection (attacker inserts messages into conversations)
    - Data exfiltration (rogue agent leaks context)
    - Denial of service (flood agent with messages to exhaust compute budget)
    - Cross-tenant data leakage
  - **Mitigations**:
    - API secret authentication (agent→backend)
    - Message-level signing: optional HMAC signature on messages for high-security channels
    - Rate limiting: per-agent, per-group, per-tag rate limits (configurable)
    - Tenant isolation: logical isolation (shared namespace) for standard tiers, namespace-per-tenant for enterprise
    - API secret rotation: generate new → grace period → invalidate old
  - **Cross-Tenant Communication** (address Metis edge case):
    - Default: fully isolated tenants
    - Opt-in: cross-tenant channels via explicit federation agreement (both tenants approve)
    - Cross-tenant messages carry source_tenant_id for audit
  - **Audit Trail**: All cross-agent messages logged with: sender, receiver, tag, timestamp, trace_id, tenant_id
  - **Data Governance** (address Metis gap):
    - Retention policies per tier (free: 7 days, premium: 90 days, enterprise: custom)
    - Data deletion API (GDPR right-to-erasure)
    - Data residency annotations (region metadata per message)

  **Must NOT do**:
  - Design the full threat model (STRIDE analysis, etc.)
  - Specify cryptographic implementations
  - Design IAM role hierarchies

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Task 13)
  - **Blocks**: Task 14
  - **Blocked By**: Tasks 4, 5

  **References**:
  - Research: No authentication standard for cross-node agent communication
  - Research: Agent impersonation, message injection, data exfiltration attack vectors
  - Research: ANP uses DID (Decentralized Identifiers) — too complex, but message signing concept is useful
  - Metis: Cross-tenant agent communication, data residency, GDPR compliance
  - Metis: Tag enforcement — broker-level vs advisory

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Threat model covers key attack vectors
    Tool: Bash (grep)
    Steps:
      1. Search for: "impersonation", "injection", "exfiltration", "denial of service"
      2. Verify each has a mitigation defined
    Expected Result: 4+ attack vectors with mitigations
    Evidence: .sisyphus/evidence/task-12-threats.txt

  Scenario: Data governance section present
    Tool: Bash (grep)
    Steps:
      1. Search for "retention", "deletion", "GDPR", "data residency"
      2. Verify retention policies defined per tier
    Expected Result: Data governance documented
    Evidence: .sisyphus/evidence/task-12-data-governance.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §11 - Security Considerations`
  - Files: `docs/design-spec.md`

- [x] 13. §12 Scalability & Performance

  **What to do**:
  - **Scale Targets**: Define concrete performance assertions:
    - 500 concurrent agents per tenant
    - 10K messages/sec throughput per tenant at p99 < 50ms broker latency
    - Agent registration/deregistration: < 100ms
    - Discovery query: < 200ms (capability search)
    - Message delivery end-to-end: < 500ms (Agent A → Broker → Agent B)
  - **Horizontal Scaling Strategy**:
    - Backend: stateless API servers behind load balancer, sharded by tenant
    - Message broker: NATS cluster with JetStream for persistence
    - Registry: read-replicated, write-primary
    - Storage: PostgreSQL with read replicas, partitioned by tenant + time
  - **Bottleneck Analysis**: Identify known bottlenecks and mitigation:
    - WebSocket connection limits per server (~10K connections per instance)
    - Message broker saturation (NATS handles 290K+/sec — sufficient for target)
    - Discovery query latency (cache agent profiles, invalidate on heartbeat miss)
    - JSON serialization overhead (acceptable for architecture overview level; binary format is Future Work)
  - **Message Ordering Guarantees** (address Metis edge case):
    - Per-group causal ordering (within a group, messages respect happens-before)
    - Cross-group: no ordering guarantee
    - Clock skew handling: server-assigned timestamps (not client timestamps) for ordering
  - **Graceful Degradation** (address Metis edge case):
    - Backend unavailable: TUI shows "disconnected" state, queues user actions
    - Broker unavailable: agents get connection error, SDK implements exponential backoff reconnect
    - Registry unavailable: agents use cached peer lists, no new discovery

  **Must NOT do**:
  - Design sharding algorithms
  - Specify hardware requirements
  - Benchmark or load test (design doc only)

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Task 12)
  - **Blocks**: Task 14
  - **Blocked By**: Task 3

  **References**:
  - Research: NATS JetStream (290K+ msgs/sec, K8s-native, 50MB footprint)
  - Research: Service mesh overhead (30-50ms per hop from sidecar proxies)
  - Research: WebSocket connection limits (10K per instance typical)
  - Metis: Clock skew between nodes — server-assigned timestamps
  - Metis: Graceful degradation when backend is down

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Performance assertions defined with concrete numbers
    Tool: Bash (grep)
    Steps:
      1. Search for latency/throughput targets (ms, msg/sec)
      2. Verify at least 5 concrete performance targets
    Expected Result: 5+ measurable performance targets
    Evidence: .sisyphus/evidence/task-13-perf-targets.txt

  Scenario: Graceful degradation documented
    Tool: Bash (grep)
    Steps:
      1. Search for "degradation", "unavailable", "disconnected"
      2. Verify behavior defined for: backend down, broker down, registry down
    Expected Result: 3 degradation scenarios documented
    Evidence: .sisyphus/evidence/task-13-degradation.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §12 - Scalability & Performance`
  - Files: `docs/design-spec.md`

- [x] 14. §13 Future Work, Open Questions & Appendices

  **What to do**:
  - **Future Work** (explicitly deferred items):
    - On-premises deployment support
    - Admin dashboard / management UI
    - Agent reputation / trust scoring system
    - Binary wire format (Protocol Buffers) for high-throughput scenarios
    - End-to-end encryption for private chats
    - Web-based TUI (via Bubble Tea WebSocket or Ratzilla WASM)
    - Agent marketplace (publish/discover third-party agents)
    - Advanced analytics (conversation insights, agent performance scoring)
    - Internationalization
  - **Open Questions** (intentionally unresolved):
    - Should custom tags require pre-registration, or can agents use any tag string?
    - What's the right default retention period for free tier?
    - Should agents be able to create groups, or only humans?
    - How should large binary attachments be handled (inline vs reference)?
    - Is on-premises deployment in v1 scope?
  - **Assumptions & Constraints** (address Metis A1-A6):
    - A1: Cross-platform Go compilation sufficient for TUI (minimum terminal: ANSI, 256-color, Unicode)
    - A2: Cloud-only for v1 (on-prem is Future Work)
    - A3: System supports persistent + ephemeral + hybrid agents
    - A4: Custom protocol is internal; external agents access via protocol adapters
    - A5: Open source (MIT/Apache) with community-first strategy; business model is open-core (paid hosting)
    - A6: Tag enforcement is hybrid — some tags broker-enforced (`no-response`), some advisory (`progress`)
    - Mark each as: VALIDATED, PENDING VALIDATION, or ASSUMPTION
  - **Appendix A: Recommended Technology Stack**:
    - TUI: Go + Bubble Tea v2, Lip Gloss, Glamour
    - Backend: Go, NATS JetStream, PostgreSQL, Redis
    - SDK: Go SDK (primary), with REST API for other languages
    - Monitoring: Prometheus + Grafana + OpenTelemetry
  - **Appendix B: Glossary** (completed version, compiled from all sections)
  - **Appendix C: Pain Point Traceability Matrix** — table mapping each of 7 pain points to the design section(s) that address it

  **Must NOT do**:
  - Design any Future Work items in detail
  - Resolve Open Questions (they're intentionally open)
  - Include deployment instructions

  **Recommended Agent Profile**:
  - **Category**: `writing`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (sole task in Wave 5)
  - **Parallel Group**: Wave 5
  - **Blocks**: Task 15
  - **Blocked By**: Tasks 2-13 (needs to reference all sections)

  **References**:
  - All previous sections (references all decisions and open items)
  - `.sisyphus/drafts/tui-im-agent-mesh.md` — full research summary
  - Metis: Assumptions A1-A6 requiring validation
  - Metis: Risks identified (protocol over-design, scope creep, document length)

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: Pain point traceability matrix present
    Tool: Bash (grep)
    Steps:
      1. Find traceability matrix in Appendix C
      2. Verify all 7 pain points have section references
    Expected Result: 7/7 pain points mapped to design sections
    Evidence: .sisyphus/evidence/task-14-traceability.txt

  Scenario: Assumptions listed with validation status
    Tool: Bash (grep)
    Steps:
      1. Count assumption entries (A1-A6)
      2. Verify each has: VALIDATED, PENDING, or ASSUMPTION marker
    Expected Result: 6 assumptions with status markers
    Evidence: .sisyphus/evidence/task-14-assumptions.txt
  ```

  **Commit**: YES
  - Message: `docs(design): add §13 - Future Work, Open Questions & Appendices`
  - Files: `docs/design-spec.md`

- [x] 15. Final Consistency Pass — Cross-References, Glossary Completion, Quality

  **What to do**:
  - **Cross-Reference Audit**: 
    - Verify all `§N` references in the document point to existing sections
    - Verify all tag names used in examples appear in the §3 taxonomy table
    - Verify all component names used throughout match §2 architecture definitions
    - Verify all metric names used match §10 definitions
  - **Glossary Completion**: 
    - Scan entire document for domain terms
    - Ensure every term used in section headers or key definitions appears in the Glossary
    - Add any missing terms found during the pass
  - **Consistency Checks**:
    - Terminology consistency (e.g., "Chat Group" vs "Group" vs "Channel" — pick one and use throughout)
    - Tag naming consistency (verify hierarchical dot notation used consistently)
    - Diagram label consistency (component names in diagrams match prose)
  - **Document Quality**:
    - Every section has: problem statement, design decision, rationale (verify)
    - No placeholder/stub text remaining
    - No TODO/FIXME without associated OPEN QUESTION
    - Target document length: 30-40 pages (trim if bloated, flag if thin)
  - **Table of Contents Update**: Ensure ToC matches actual section headers and numbering
  - **Metadata Update**: Update version to 1.0.0, status to "Ready for Review"

  **Must NOT do**:
  - Add new content or sections
  - Change design decisions
  - Resolve open questions

  **Recommended Agent Profile**:
  - **Category**: `deep`
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (sole task in Wave 6)
  - **Parallel Group**: Wave 6
  - **Blocks**: F1, F2, F3, F4
  - **Blocked By**: Tasks 1-14

  **References**:
  - The complete `docs/design-spec.md` document (all sections)
  - Plan's "Must Have" and "Must NOT Have" checklists

  **Acceptance Criteria**:

  **QA Scenarios (MANDATORY):**

  ```
  Scenario: No broken cross-references
    Tool: Bash (grep)
    Steps:
      1. Extract all §N references from the document
      2. Verify each points to an existing section header
    Expected Result: Zero broken references
    Evidence: .sisyphus/evidence/task-15-cross-refs.txt

  Scenario: All tags in examples exist in taxonomy
    Tool: Bash (grep)
    Steps:
      1. Extract all tag names from JSON examples
      2. Cross-reference against §3 taxonomy table
      3. Report any tags used but not defined
    Expected Result: Zero undefined tags in examples
    Evidence: .sisyphus/evidence/task-15-tag-consistency.txt

  Scenario: No stub/placeholder text remaining
    Tool: Bash (grep)
    Steps:
      1. Search for: "TODO", "FIXME", "TBD", "placeholder", "[insert"
      2. Verify zero results (or all are intentional OPEN QUESTIONs)
    Expected Result: Zero unintentional stubs
    Evidence: .sisyphus/evidence/task-15-no-stubs.txt

  Scenario: Document length in target range
    Tool: Bash (wc)
    Steps:
      1. Count words in docs/design-spec.md
      2. Estimate pages (250 words/page)
      3. Verify 30-40 page range (7,500-10,000 words)
    Expected Result: Document is 7,500-10,000 words
    Evidence: .sisyphus/evidence/task-15-word-count.txt
  ```

  **Commit**: YES
  - Message: `docs(design): finalize cross-references, glossary & consistency pass`
  - Files: `docs/design-spec.md`

---

## Final Verification Wave (MANDATORY — after ALL implementation tasks)

> 4 review agents run in PARALLEL. ALL must APPROVE. Rejection → fix → re-run.

- [x] F1. **Plan Compliance Audit** — dispatch via `task(subagent_type="oracle", ...)`
  Read the full design spec at `docs/design-spec.md`. For each "Must Have": verify the section exists and has substantive content (not placeholder). For each "Must NOT Have": search document for forbidden content (runnable code, SDK API details, billing design, deployment infra). Verify all 7 pain points have traceable design decisions. Check evidence files exist in `.sisyphus/evidence/`.
  Output: `Must Have [N/N] | Must NOT Have [N/N] | Pain Points [7/7] | VERDICT: APPROVE/REJECT`

- [x] F2. **Document Quality Review** — `unspecified-high`
  Run markdownlint or equivalent. Check: broken Mermaid diagrams, invalid cross-references, inconsistent terminology, sections without problem statements, missing rationale for design decisions. Check for AI slop: excessive repetition, vague language ("should work well"), placeholder text.
  Output: `Sections [N/N complete] | Diagrams [N/N valid] | Terms [consistent/N issues] | VERDICT`

- [x] F3. **Pain Point Traceability** — `deep`
  For each of the 7 validated pain points, trace through the document to find: (1) where it's mentioned as a problem, (2) which design decision addresses it, (3) whether the solution is concrete or vague. Score each pain point as FULLY_ADDRESSED / PARTIALLY_ADDRESSED / NOT_ADDRESSED.
  Output: `Pain Points [N/7 fully | N/7 partially | N/7 not addressed] | VERDICT`

- [x] F4. **Scope Fidelity Check** — `deep`
  For each section: verify content matches the plan's description. Check for scope creep (sections that go beyond architecture overview into implementation). Check for missing content (sections that are too thin). Verify "Must NOT Have" guardrails are respected throughout.
  Output: `Sections [N/N compliant] | Scope Creep [CLEAN/N issues] | VERDICT`

---

## Commit Strategy

- **1**: `docs(design): scaffold BobberChat design spec skeleton` — docs/design-spec.md
- **2**: `docs(design): add §1 - Executive Summary & Problem Statement` — docs/design-spec.md
- **3**: `docs(design): add §2 - System Architecture Overview` — docs/design-spec.md
- **4**: `docs(design): add §3 - Custom Protocol & Message Tag Taxonomy` — docs/design-spec.md
- **5**: `docs(design): add §5 - Identity, Authentication & Agent Lifecycle` — docs/design-spec.md
- **6**: `docs(design): add §4 - Conversation Model` — docs/design-spec.md
- **7**: `docs(design): add §6 - Agent Discovery & Registry` — docs/design-spec.md
- **8**: `docs(design): add §7 - Approval Workflows & Coordination` — docs/design-spec.md
- **9**: `docs(design): add §8 - Protocol Adapters (MCP/A2A/gRPC)` — docs/design-spec.md
- **10**: `docs(design): add §9 - TUI Client Design & Layout` — docs/design-spec.md
- **11**: `docs(design): add §10 - Observability & Debugging` — docs/design-spec.md
- **12**: `docs(design): add §11 - Security Considerations` — docs/design-spec.md
- **13**: `docs(design): add §12 - Scalability & Performance` — docs/design-spec.md
- **14**: `docs(design): add §13 - Future Work, Open Questions & Appendices` — docs/design-spec.md
- **15**: `docs(design): finalize cross-references, glossary & consistency pass` — docs/design-spec.md

---

## Success Criteria

### Final Checklist
- [ ] All 13 sections have substantive content (no stubs)
- [ ] Core message tag taxonomy (~15 tags) fully defined
- [ ] At least 5 diagrams present and rendering correctly
- [ ] Every section has problem statement + design decision + rationale
- [ ] All 7 pain points traceable to specific design decisions
- [ ] Glossary complete with all domain terms
- [ ] No broken cross-references
- [ ] "Must NOT Have" guardrails fully respected
- [ ] Document is 30-40 pages (not bloated, not thin)
- [ ] All OPEN QUESTION markers are intentional, not missed decisions
