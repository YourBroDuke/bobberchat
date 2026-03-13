# BobberChat Design Specification - Learnings

## Task 1: Document Skeleton Execution

### Patterns & Conventions

1. **Section Numbering Format**: Use `## § N. Title` format for main sections (H2 headers). This provides clear visual hierarchy and matches the plan's numbering scheme.

2. **Glossary Structure**: Glossary terms use `### **Term**` (H3 header) for each term, followed by "TBD" placeholder. This creates clickable markdown links in GitHub and maintains consistency.

3. **Metadata Header Format**: YAML front matter works well for document metadata:
   ```yaml
   ---
   title: BobberChat Design Specification
   version: 0.1.0
   status: Draft
   date: 2026-03-13
   authors: BobberChat Team
   ---
   ```

4. **Notation & Conventions**: Three subsections (RFC 2119, OPEN QUESTION, Diagram Notation) are sufficient to guide readers on document style.

### Successful Approaches

1. **Table of Contents with Anchors**: Including a full ToC with markdown links to sections helps readers navigate the document structure before diving in.

2. **Audience-First Introduction**: "How to Read This Document" section with 4 specific audience types + guidance on which sections to focus on prevents context-switching waste.

3. **Horizontal Rules for Separation**: `---` (markdown horizontal rule) clearly separates major document sections and improves readability.

4. **Appendices Planning**: Including empty appendix structure (A, B, C) signals planned organization for supporting materials without cluttering main content.

### QA Verification Insights

1. **Section Header Counting**: `grep "^## §"` reliably counts main sections. This pattern is specific enough to avoid false positives (matches only H2 headers starting with "§").

2. **Glossary Term Extraction**: Use `sed -n '/## Glossary/,/^## Appendices/p'` to isolate glossary section, then `grep "^### \*\*"` to count term headers. This is more reliable than counting all bold text (which can appear elsewhere).

3. **Metadata Verification**: Checking first 20 lines with `head -n 20` catches YAML front matter before main content starts.

4. **Line Count Baseline**: 215 lines for skeleton provides good baseline for future content growth tracking.

### Design Decisions

1. **Version Starting at 0.1.0**: Reflects that this is a draft specification subject to change. Follows semver convention (major.minor.patch).

2. **Status: Draft**: Clear signal to readers and downstream tools that content is incomplete and evolving.

3. **Author Attribution to Team**: Matches collaborative nature of multi-task specification writing.

### Potential Blockers for Wave 2

- Content writers will need to understand that section bodies should start after "TBD" placeholder
- Glossary stubs ("TBD") will need to be replaced with actual definitions by end of final pass (Task 15)
- Cross-references between sections will need to be validated in Task 15

- **Architecture Patterns**: For Section §2, defined a 3-component split (Backend, SDK, TUI) to handle agent-specific messaging separate from human observability.
- **NATS JetStream**: Integrated NATS as the high-throughput message bus to handle 290K+ msgs/sec requirements.
- **Loop Prevention Awareness**: Documented how message tags and SDK abstraction can facilitate loop prevention, a key identified pain point.
- **TUI Integration**: Defined Bubble Tea v2 for the TUI Client to ensure consistency with current Go-based AI agent tooling (like k9s).
- Documented § 1 Executive Summary & Problem Statement with 7 validated pain points and evidence (AgentRx, LangGraph, developer survey).
- Linked product features directly to pain points in 'Why BobberChat?' subsection.
- Integrated market context highlighting the 40% failure rate in production multi-agent systems.
- Contrasted BobberChat with existing TUI and monitoring tools in 'Competitive Landscape'.

## Task 5: §5 Identity, Authentication & Agent Lifecycle

- Use a two-principal identity model: human account principal (`email -> user`) and agent workload principal (`agent_id`) linked by `owner_user_id`.
- Keep SaaS tiering statement in spec-level language only (free tier limited to N agents, premium unlimited) without entering billing implementation details.
- A clear split between Agent auth (API secret) and TUI auth (JWT) prevents conflating machine credentials with user session tokens.
- Include explicit WebSocket upgrade auth in diagrams/text; this is easy to miss when specs only describe REST authentication.
- Lifecycle coverage is strongest when documenting all three models (persistent/ephemeral/hybrid) plus a shared canonical state machine.
- API secret rotation should explicitly state grace-period dual-validity and eventual hard invalidation of old secret.
- BobberChat-native Agent Card should include routing-critical fields (`capabilities`, `supported_tags`, `endpoints`) and publish/update behavior.

## Task 4: Protocol Design and Tag Taxonomy

- Replacing section-level `TBD` with structured subsections (`3.1`-`3.8`) keeps the spec navigable even when content is dense and normative.
- A canonical JSON envelope plus field table avoids ambiguity between transport fields (`id`, `trace_id`) and tag-specific business payload.
- Taxonomy tables are most useful when each row includes both runtime semantics (delivery guarantee) and enforcement semantics (what broker must do).
- Explicit loop-prevention tags (`context-provide`, `no-response`) should be paired with broker circuit-breaker behavior, not left as sender-side conventions.
- Documenting per-family guarantees (`request.*`, `progress.*`, `approval.*`) reduces implementation drift across SDKs/adapters.
- Adding `context-budget` metadata in protocol spec creates a direct mechanism to control token amplification and context-window pressure.
### Conversation Model Patterns
- **Three-Tier Persistence**: Balancing real-time performance with Redis (Hot, 3h), thread reconstruction with Postgres (Warm, 30d), and auditability with Object Storage (Cold, 90d+).
- **Topic-Centric Threading**: Auto-creation of topics based on  tags and subject names to ensure task context is always grouped.
- **Delivery Semantics**: Private chats use at-least-once delivery with receipt acknowledgment, while groups use broadcast-to-online with retention-based queuing for offline members.
### Conversation Model Patterns
- **Three-Tier Persistence**: Balancing real-time performance with Redis (Hot, 3h), thread reconstruction with Postgres (Warm, 30d), and auditability with Object Storage (Cold, 90d+).
- **Topic-Centric Threading**: Auto-creation of topics based on `request.*` tags and subject names to ensure task context is always grouped.
- **Delivery Semantics**: Private chats use at-least-once delivery with receipt acknowledgment, while groups use broadcast-to-online with retention-based queuing for offline members.

## Task 6: §6 Agent Discovery & Registry

- Define a centralized registry in the backend with fields for capabilities, supported tags, version, and status.
- Registry data model includes 'connected_at' and 'last_heartbeat' for health monitoring.
- Default heartbeat interval of 30 seconds with 3-interval timeout policy for transition to OFFLINE.
- Capability-based routing allows requesters to target 'capability:name' instead of specific 'agent_id'.
- Discovery query API supports semantic search by capability, tag support, status, and owner.
- Ephemeral agent churn handled via registration rate limiting and profile caching.
- Mermaid sequence diagram created to illustrate the publish-query-route flow.

## Task 9: §8 Protocol Adapters (MCP/A2A/gRPC Bridging)

- Use a protocol-agnostic adapter contract table (`Input`, `Transform`, `Output`, `Reverse Transform`, `Validation`) so each adapter section reuses one normalization model.
- Adapter mapping tables are strongest when they include both directions (external→BobberChat and BobberChat→external), not only inbound translation.
- MCP bridge should explicitly document synthetic `agent_id` assignment because MCP lacks native agent identity semantics.
- A2A bridge should map not only messaging (`message/send`) but also discovery (Agent Cards ↔ Agent Profiles) and workflow state (tasks ↔ Topics).
- gRPC bridge should separate unary and streaming semantics: unary maps to `request.action`/`response.*`, streaming maps to `progress.*` plus terminal response/error.
- Tag auto-mapping rules benefit from precedence ordering (explicit table rules > generic transport heuristics > safe fallback `context-provide`) to avoid accidental loop-causing tags.
- Adapter lifecycle clarity improves when framed as backend plugin stages: startup registration, capability advertisement, active translation, health reporting, hot disable/upgrade.

## Task 8: §7 Approval Workflows & Coordination Primitives

- **HITL Integration**: Established a structured push-based approval model (`approval.request`) which contrasts with pull-based elicitation common in other protocols.
- **Escalation Logic**: Documented a fallback chain (Primary -> Secondary -> Human) to prevent system stalls, a critical requirement for autonomous agent reliability.
- **Coordination Primitives**: Defined four core primitives (Priority, Voting, Arbiter, Escalation-to-Human) to handle resource contention and multi-agent decision making.
- **Safety Mechanics**: Integrated `max_cost` budgeting and circuit breaker patterns directly into the coordination layer to address identified pain points like token-cost explosions and infinite retries.
- **Flow Visualization**: Used Mermaid `sequenceDiagram` to illustrate complex HITL scenarios (happy path, timeout, rejection), providing clearer guidance for TUI and SDK implementors.

## § 10 Observability & Debugging
- **Tracing**: Messages carry `trace_id` and `parent_span_id`.
- **Naming**: Span naming follows `agent:{agent_id}:{tag}`.
- **Metrics**: 6 key metrics defined: messages.sent (Counter), messages.latency_ms (Histogram), agents.online (Gauge), topics.active (Gauge), approvals.pending (Gauge), errors.count (Counter).
- **Features**: Message replay, conversation trace, state diff, and dependency graph.
- **Alerting**: Failure-pattern alerts delivered in TUI.

### TUI Client Design (Task 10)
* Established a three-pane layout for the TUI: Agent Directory (left), Message View (center), Context Panel (right).
* Handled high density scaling via grouping, aggregation, and specific notification priorities (CRITICAL, INFO, DEBUG).
* Defined key views and interactions (Vim bindings, command palette, and `<120 cols` responsive rules) avoiding widget-level specifics.

## Task 12: §11 Security Considerations

- **Threat Table Pattern**: Used a markdown table to pair attack vectors with mitigations for high-density information delivery.
- **RFC 2119 for Security**: Applied MUST/SHOULD keywords consistently to differentiate between mandatory security controls (authentication) and optional ones (message signing).
- **Tenant Isolation**: Established logical isolation as the default state with explicit opt-in for federation, addressing cross-tenant leakage concerns.
- **Data Governance Alignment**: Mapped retention policies directly to service tiers (Free/Premium/Enterprise) to ensure the spec supports business model requirements.
- **Audit Traceability**: Linked audit requirements to `trace_id` and `tenant_id` to ensure observability tools can reconstruct security events across distributed agents.

## Task 13: §12 Scalability & Performance

- **Performance Assertions**: Defined 5+ concrete targets including concurrent agents (500), throughput (10K msg/sec), and various latency metrics (broker < 50ms, registration < 100ms, discovery < 200ms, end-to-end < 500ms).
- **Horizontal Scaling**: Documented sharding by tenant for API, NATS JetStream for broker, read-replication for registry, and partitioning for PostgreSQL.
- **Bottleneck Management**: Addressed WebSocket limits (~10K/instance), broker saturation (NATS 290K+/sec capacity), discovery latency (caching), and JSON overhead.
- **Ordering Guarantees**: Defined per-group causal ordering, cross-group unordered, and server-side timestamps for clock skew mitigation.
- **Graceful Degradation**: Documented behavior for backend, broker, and registry failures, emphasizing SDK queuing and local caching.
- **Reference Integration**: Linked to §2 (NATS performance), §6 (discovery caching), and Metis research (clock skew and race conditions).

## Task 14: §13 Future Work, Open Questions & Appendices

- **Appendix Organization**: Used a table for Appendix B (Acronyms) to maximize vertical space and readability.
- **Synthesized References**: Mapped research citations from drafts directly to product features (e.g., AgentRx -> Observability).
- **Open Question Extraction**: Consolidated open questions from the entire document into §13.2 while maintaining their original context via cross-references.
- **JSON Example Diversification**: Included 6 distinct message types in Appendix C to provide a complete "day in the life" of the protocol (Request, Response, Progress, Approval, Error, Context).
- **Future Work Scaling**: Focused deferred items on enterprise/scale features (Multi-region, E2EE, Federation) to signal the spec's long-term roadmap.

## Task 15: Final Consistency Pass

- **Evidence File Robustness**: Cross-reference evidence should include an explicit broken-reference count from `comm -23` output; listing all refs/sections alone can hide false positives.
- **Glossary Closure Pattern**: Final-pass glossary quality improves when canonical component terms are explicitly mirrored as glossary entries (e.g., `Backend Service`, `Agent SDK/CLI`, `TUI Client`) in addition to shorthand aliases.
- **Protocol Example Consistency**: Appendix protocol examples should preserve canonical envelope fields (`id`, `from`, `to`, `tag`, `payload`, `metadata`, `timestamp`, `trace_id`) to avoid drift from §3 semantics.
- **Section Quality Gate**: Adding explicit section-level triads (`Problem Statement`, `Design Decision`, `Rationale`) is an effective final-review structure check for §1–§13 without changing architecture scope.
- **Terminology Stabilization**: Defining canonical terms in a dedicated conventions subsection prevents recurring drift between "Chat Group" and "Channel" across prose and diagrams.
