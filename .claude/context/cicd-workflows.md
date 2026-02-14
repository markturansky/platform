# CI/CD Workflows Reference

**When to load:** Working on GitHub Actions, CI pipelines, or deployment automation

## Component Build Pipeline (`.github/workflows/components-build-deploy.yml`)

- **Change detection**: Only builds modified components (frontend, backend, operator, claude-runner)
- **Multi-platform builds**: linux/amd64 and linux/arm64
- **Registry**: Pushes to `quay.io/ambient_code` on main branch
- **PR builds**: Build-only, no push on pull requests

## Automation Workflows

- **amber-issue-handler.yml**: Amber background agent - automated fixes via GitHub issue labels (`amber:auto-fix`, `amber:refactor`, `amber:test-coverage`) or `/amber execute` command
- **amber-dependency-sync.yml**: Daily sync of dependency versions to Amber agent knowledge base
- **claude.yml**: Claude Code integration - responds to `@claude` mentions in issues/PRs
- **claude-code-review.yml**: Automated code reviews on pull requests

## Code Quality Workflows

- **go-lint.yml**: Go code formatting, vetting, and linting (gofmt, go vet, golangci-lint)
- **frontend-lint.yml**: Frontend code quality (ESLint, TypeScript checking, build validation)

## Deployment & Testing Workflows

- **prod-release-deploy.yaml**: Production releases with semver versioning and changelog generation
- **e2e.yml**: End-to-end Cypress testing in kind cluster
- **test-local-dev.yml**: Local development environment validation

## Utility Workflows

- **docs.yml**: Deploy MkDocs documentation to GitHub Pages
- **dependabot-auto-merge.yml**: Auto-approve and merge Dependabot dependency updates
