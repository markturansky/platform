# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The **Ambient Code Platform** is a Kubernetes-native AI automation platform that orchestrates intelligent agentic sessions through containerized microservices. The platform enables AI-powered automation for analysis, research, development, and content creation tasks via a modern web interface.

> **Note:** This project was formerly known as "vTeam". Technical artifacts (image names, namespaces, API groups, routes) still use "vteam" for backward compatibility. The docs use ACP naming.

### Amber Background Agent

Amber automates common development tasks via GitHub Issues. See [Amber Quickstart](docs/amber-quickstart.md), [Full Docs](docs/amber-automation.md), [Config](.claude/amber-config.yml).

Labels: `amber:auto-fix` (formatting/linting), `amber:refactor` (break large files), `amber:test-coverage` (add tests).

### Core Architecture

```
User Creates Session → Backend Creates CR → Operator Spawns Job →
Pod Runs Claude CLI → Results Stored in CR → UI Displays Progress
```

1. **Frontend** (NextJS + Shadcn): Web UI for session management and monitoring
2. **Backend API** (Go + Gin): REST API managing Kubernetes Custom Resources with multi-tenant project isolation
3. **Agentic Operator** (Go): Kubernetes controller watching CRs and creating Jobs
4. **Claude Code Runner** (Python): Job pods executing Claude Code CLI with multi-agent collaboration

### Custom Resource Definitions

1. **AgenticSession** (`agenticsessions.vteam.ambient-code`): AI execution session with prompt, repos, interactive mode, timeout, model selection
2. **ProjectSettings** (`projectsettings.vteam.ambient-code`): Project-scoped config (API keys, models, timeouts)
3. **RFEWorkflow** (`rfeworkflows.vteam.ambient-code`): 7-step agent council for engineering refinement

### Key Concepts

- **Multi-Repo**: Sessions operate on multiple repos; `mainRepoIndex` selects working directory
- **Interactive vs Batch**: Batch (default) = single prompt; Interactive = long-running chat via inbox/outbox
- **Project Isolation**: Each project maps to a K8s namespace with RBAC enforcement

## Loadable Context System

Load targeted context files instead of relying on this file. **Always load the relevant context file before starting work.**

### Context Routing Table

| Task | Context File | Pattern File |
|------|-------------|-------------|
| Backend API / Operator work | `.claude/context/backend-development.md` | `.claude/patterns/k8s-client-usage.md`, `.claude/patterns/error-handling.md` |
| Frontend UI work | `.claude/context/frontend-development.md` | `.claude/patterns/react-query-usage.md` |
| Security review | `.claude/context/security-standards.md` | `.claude/patterns/error-handling.md` |
| Build / Deploy / Local dev | `.claude/context/dev-commands.md` | — |
| Configuration / Standards | `.claude/context/config-standards.md` | — |
| CI/CD / GitHub Actions | `.claude/context/cicd-workflows.md` | — |
| Testing | `.claude/context/testing-strategy.md` | — |
| Langfuse / Observability | — | `.claude/patterns/langfuse-observability.md` |
| Architecture questions | Load `repomix-analysis/03-architecture-only.xml` | See ADRs below |

### Architectural Decision Records (`docs/adr/`)

- `0001-kubernetes-native-architecture.md`
- `0002-user-token-authentication.md`
- `0003-multi-repo-support.md`
- `0004-go-backend-python-runner.md`
- `0005-nextjs-shadcn-react-query.md`

### Repomix Architecture View

Single view: `repomix-analysis/03-architecture-only.xml` (187K tokens, grade 8.8/10). See `.claude/repomix-guide.md`.

### Decision Log

`docs/decisions.md` — Lightweight chronological record linking to ADRs and code.

## Quick Command Reference

```bash
make kind-up           # Local dev with Kind
make build-all         # Build all container images
make deploy            # Deploy to cluster
make test              # Run tests
make lint              # Lint code
make clean             # Clean up deployment
```

For full command reference, load `.claude/context/dev-commands.md`.

## Essential Standards (Always Apply)

### Go (Backend & Operator)

- `go fmt ./...` enforced; `golangci-lint run` required
- Table-driven tests with subtests
- No `panic()` in production code
- Always use `GetK8sClientsForRequest(c)` for user operations
- Never log tokens (use `len(token)` instead)

For complete patterns, load `.claude/context/backend-development.md`.

### Frontend (NextJS)

- Zero `any` types
- Shadcn UI components only (`@/components/ui/*`)
- React Query for ALL data operations (`@/services/queries/*`)
- `type` over `interface`
- `npm run build` must pass with 0 errors, 0 warnings

For complete patterns, load `.claude/context/frontend-development.md` and `components/frontend/DESIGN_GUIDELINES.md`.

### Python (Runner)

- `uv` over `pip`; black formatting; isort with black profile; flake8

### Git Workflow

- Default branch: `main`
- Conventional commits (squashed on merge)
- Always check current branch before file modifications

### Kubernetes/OpenShift

- Default namespace: `ambient-code` (prod), `vteam-dev` (local dev)
- CRD group: `vteam.ambient-code`, API version: `v1alpha1`

## Production Considerations

- **Health**: `/health` on backend API
- **Monitoring**: Structured logging, Prometheus-compatible metrics, K8s events
- **Scaling**: HPA by CPU/memory, operator-managed job concurrency, namespace-isolated multi-tenancy

## Documentation Standards

- Default to improving existing docs rather than creating new files
- Colocate docs with relevant code (e.g., `components/backend/README.md`)
- Only create top-level docs for cross-cutting concerns

## Component READMEs

- **Backend**: `components/backend/README.md`
- **Frontend**: `components/frontend/README.md`, `DESIGN_GUIDELINES.md`, `COMPONENT_PATTERNS.md`
- **Operator**: `components/operator/README.md`
- **Runner**: `components/runners/claude-code-runner/README.md`
