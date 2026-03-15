# AGENTS.md — Project-Level AI Agent Rules

## Post-Completion Rule: Documentation Review & Commit

**After completing any implementation work**, the agent MUST perform the following steps before reporting completion:

### 1. Review Documentation for Staleness

Check whether the changes made require updates to any of these documents:

| Document | What to check |
|----------|--------------|
| `README.md` | API endpoint table, CLI flags, keybindings, env vars, quick start instructions |
| `docs/PROJECT_STATUS.md` | "What's Done" sections, file/line counts, test counts, file tree, dependency versions |
| `docs/tech-design.md` | Architecture descriptions, component interactions, data flows |
| `docs/design-spec.md` | Protocol definitions, interface contracts, system specifications |
| `docs/prd.md` | Feature descriptions, requirements traceability |
| `api/openapi/openapi.yaml` | Endpoint paths, request/response schemas, status codes |
| `docs/tsg/*.md` | Deployment steps, config references, troubleshooting entries |

### 2. Update Affected Documents

- Update any document where the completed work has introduced new information, changed existing behavior, or made content inaccurate.
- Keep the same style and formatting as the existing document.
- Update the `Last updated` date in `docs/PROJECT_STATUS.md` if it was modified.

### 3. Commit and Push

After all code changes AND documentation updates are staged:

```bash
git add -A
git commit -m "<descriptive message covering both code and doc changes>"
git push origin <current-branch>
```

- Use a single commit that includes both the implementation and documentation updates.
- The commit message should mention doc updates if non-trivial (e.g., "Add X feature; update README and PROJECT_STATUS").
- Push to the current branch's remote tracking branch.

### Rule Applicability

This rule applies when:
- New features, endpoints, or CLI commands are added
- Existing behavior is changed or removed
- Tests are added or test counts change significantly
- Dependencies are added or upgraded
- Infrastructure or deployment config changes
- Bug fixes that affect documented behavior

This rule does NOT apply when:
- The change is purely internal refactoring with no external-facing impact
- Only whitespace, formatting, or comment changes are made
- The user explicitly opts out (e.g., "skip docs", "no commit")
