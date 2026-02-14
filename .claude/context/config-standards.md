# Configuration Standards

**When to load:** Setting up environments, configuring components, or working with container images

## Python

- **Virtual environments**: Always use `python -m venv venv` or `uv venv`
- **Package manager**: Prefer `uv` over `pip`
- **Formatting**: black (double quotes)
- **Import sorting**: isort with black profile
- **Linting**: flake8 (ignore E203, W503)

## Go

- **Formatting**: `go fmt ./...` (enforced)
- **Linting**: golangci-lint (install via `make install-tools`)
- **Testing**: Table-driven tests with subtests
- **Error handling**: Explicit error returns, no panic in production code

## Container Images

- **Default registry**: `quay.io/ambient_code`
- **Image tags**: Component-specific (vteam_frontend, vteam_backend, vteam_operator, vteam_claude_runner)
- **Platform**: Default `linux/amd64`, ARM64 supported via `PLATFORM=linux/arm64`
- **Build tool**: Docker or Podman (`CONTAINER_ENGINE=podman`)

## Git Workflow

- **Default branch**: `main`
- **Feature branches**: Required for development
- **Commit style**: Conventional commits (squashed on merge)
- **Branch verification**: Always check current branch before file modifications

## Kubernetes/OpenShift

- **Default namespace**: `ambient-code` (production), `vteam-dev` (local dev)
- **CRD group**: `vteam.ambient-code`
- **API version**: `v1alpha1` (current)
- **RBAC**: Namespace-scoped service accounts with minimal permissions

## Documentation Standards

**Default to improving existing documentation** rather than creating new files.

- **Prefer inline updates**: Improve existing markdown files or code comments
- **Colocate new docs**: Documentation should live in the subdirectory with relevant code (e.g., `components/backend/README.md`) not at the top level
- **Avoid top-level proliferation**: Only create top-level docs for cross-cutting concerns
- **Follow established patterns**: See `docs/amber-quickstart.md` and `components/backend/README.md`
