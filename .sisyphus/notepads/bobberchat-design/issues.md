# Issues — BobberChat Design

> Problems, gotchas, and edge cases encountered.

---
- 2026-03-13 F1 compliance audit: 9/9 must-haves found, but guardrail violations remain in tier/pricing content, admin dashboard mention, deployment infrastructure detail, 24-tag core catalog, exact version references, and TUI interaction details. Verdict: REJECT.
- 2026-03-13 F4 scope fidelity: §3 exceeds planned core tag scope (24 vs ~15), §6 lacks explicit endpoint-level API sketch, and §13 misses required assumptions block (A1-A6 + status) plus pain-point traceability matrix appendix. Verdict: REJECT.
- 2026-03-13 F1 rerun: Must-have coverage remains 9/9 and pain-point traceability 7/7, but compliance still REJECTED. Remaining guardrail violations are deployment infrastructure detail (K8s/cluster/load balancer/on-prem references), exact version references (Bubble Tea v2, sdk_version 0.6, A2A v1), and UI specifics (keyboard navigation/command interface/view switching, compact mode threshold, ANSI/256-color assumptions).
- 2026-03-13 F1 final rerun: Document now passes 9/9 must-haves, 7/7 pain-point checks, and 7/8 guardrails. Remaining blocker is deployment-infrastructure language in scalability/assumptions sections (clustered brokering, horizontally deployed instances, replication/replicas, cloud-only deployment assumption). Verdict remains REJECT.
- 2026-03-13 F4 re-execution: all prior F4 blockers are resolved in current 1693-line spec; section fidelity and guardrails are now clean with APPROVE verdict recorded in `task-F4-scope-fidelity-PASS.txt`.
