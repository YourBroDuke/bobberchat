# CI/CD Pipeline

BobberChat uses GitHub Actions with two separate deployment paths: staging (automatic on merge) and production (manual via release tag).

## Pipeline Overview

```
Push to master ──> CI ──> Docker push (SHA tag) ──> Deploy to Staging
Push tag v*    ──> Release (semver tag + binaries) ──> Deploy to Production
```

## Workflow Files

| File | Trigger | Purpose |
| --- | --- | --- |
| `ci.yml` | Push/PR to master | Lint, build, test, integration, e2e, Docker push |
| `deploy-staging.yml` | CI completion on master | Helm deploy to AKS staging |
| `release.yml` | Push tag `v*` | Docker image (semver), binaries, GitHub Release |
| `deploy-production.yml` | Release completion | Helm deploy to AKS production |

## Staging Pipeline

Triggered automatically when code is pushed or merged into `master`.

1. **CI** (`ci.yml`) runs lint, build, unit tests, integration tests, e2e tests, and Docker build validation — all in parallel.
2. After all checks pass, the `docker-push` job builds and pushes the image to `ghcr.io/yourbroduke/bobberchat` tagged with the **git SHA** (7-char) and `latest`.
3. **Deploy to Staging** (`deploy-staging.yml`) triggers via `workflow_run` on CI completion. It deploys to AKS staging using the SHA tag and runs a health check against `https://staging.bobbers.cc/v1/health`.

Pull requests also trigger CI checks (steps 1 only) but do **not** push images or deploy.

### Image Tag Format (Staging)

```
ghcr.io/yourbroduke/bobberchat:abc1234
ghcr.io/yourbroduke/bobberchat:latest
```

## Production Pipeline

Triggered when a release tag is pushed.

1. **Release** (`release.yml`) triggers on tags matching `v*`. It builds and pushes the Docker image with **semver tags** (`1.2.3`, `1.2`), builds cross-platform binaries, and creates a GitHub Release with auto-generated release notes.
2. **Deploy to Production** (`deploy-production.yml`) triggers via `workflow_run` on Release completion. It deploys to AKS production using the semver tag (without `v` prefix) and runs a health check against `https://api.bobbers.cc/v1/health`.

### Image Tag Format (Production)

```
ghcr.io/yourbroduke/bobberchat:1.2.3
ghcr.io/yourbroduke/bobberchat:1.2
```

### Creating a Release

```bash
git tag v1.2.3
git push origin v1.2.3
```

## Manual Deploys

Both deploy workflows support `workflow_dispatch` for emergency or ad-hoc deployments. Trigger manually from the GitHub Actions UI:

- **Actions > Deploy to Staging > Run workflow**
- **Actions > Deploy to Production > Run workflow**

Manual staging deploys use the current `master` HEAD SHA as the image tag. Manual production deploys use the current branch/tag name (strip `v` prefix).

## Required GitHub Secrets

Configure in **Settings > Secrets and variables > Actions**:

### Azure OIDC (shared across environments)

| Secret | Description |
| --- | --- |
| `AZURE_CLIENT_ID` | Service Principal Application ID |
| `AZURE_TENANT_ID` | Azure AD Tenant ID |
| `AZURE_SUBSCRIPTION_ID` | Azure Subscription ID |

### Staging Environment

Set these under the `staging` environment in **Settings > Environments**:

| Secret | Description |
| --- | --- |
| `STAGING_POSTGRES_HOST` | Azure PostgreSQL FQDN |
| `STAGING_POSTGRES_PASSWORD` | PostgreSQL admin password |
| `STAGING_POSTGRES_DSN` | Full connection string (`postgres://...?sslmode=require`) |
| `STAGING_JWT_SECRET` | JWT signing secret (256-bit random) |

### Production Environment

Set these under the `production` environment in **Settings > Environments**:

| Secret | Description |
| --- | --- |
| `PRODUCTION_POSTGRES_HOST` | Azure PostgreSQL FQDN |
| `PRODUCTION_POSTGRES_PASSWORD` | PostgreSQL admin password |
| `PRODUCTION_POSTGRES_DSN` | Full connection string (`postgres://...?sslmode=require`) |
| `PRODUCTION_JWT_SECRET` | JWT signing secret (256-bit random) |

## Environment Protection Rules

Configure in **Settings > Environments**:

- **staging**: No approval required (auto-deploy on merge).
- **production**: Recommended to add required reviewers for manual approval before deploy.

## Troubleshooting

### Staging deploy did not trigger after merge

Check that the CI workflow completed **successfully**. The `deploy-staging.yml` workflow only runs when CI reports `conclusion == 'success'`. If any CI job failed, fix the failure and push again.

```bash
gh run list --workflow=ci.yml --branch=master --limit=5
```

### Production deploy did not trigger after tagging

Check that the Release workflow completed successfully:

```bash
gh run list --workflow=release.yml --limit=5
```

### Wrong image tag deployed

For staging, the tag is the first 7 characters of the commit SHA. Verify with:

```bash
git rev-parse --short=7 HEAD
```

For production, the tag is the version number without the `v` prefix (e.g., tag `v1.2.3` deploys image `1.2.3`).

### Manual deploy uses wrong image

Manual `workflow_dispatch` for staging uses `github.sha` of the workflow run (current HEAD of default branch). For production, it uses `github.ref_name` (the branch or tag name you're on when triggering).
