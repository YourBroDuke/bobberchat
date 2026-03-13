# Decisions — BobberChat Design

> Architectural choices, trade-offs, and rationale.

---

- 2026-03-13: F4 scope-fidelity rerun uses `.sisyphus/evidence/task-F4-scope-fidelity-PASS.txt` as canonical output target with per-section line-cited findings and explicit 8-guardrail disposition.
- 2026-03-13: Treat contextual/non-design occurrences (e.g., string literals in JSON examples) as non-violations for deployment-infrastructure guardrail unless they prescribe architecture or operations.
