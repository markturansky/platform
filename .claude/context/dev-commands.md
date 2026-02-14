# Development Commands Reference

**When to load:** Setting up local dev environment, building, deploying, or debugging

## Quick Start - Local Development

**Recommended: Kind (Kubernetes in Docker):**

```bash
# Prerequisites: Docker installed
# Fast startup, matches CI environment
make kind-up

# Access at http://localhost:8080
# Full guide: docs/developer/local-development/kind.md
```

**Alternative: OpenShift Local (CRC) - for OpenShift-specific features:**

```bash
# Prerequisites: brew install crc
# Get free Red Hat pull secret from console.redhat.com/openshift/create/local
make dev-start

# Access at https://vteam-frontend-vteam-dev.apps-crc.testing
```

**Hot-reloading development:**

```bash
# Terminal 1
DEV_MODE=true make dev-start

# Terminal 2 (separate terminal)
make dev-sync
```

## Building Components

```bash
# Build all container images (default: docker, linux/amd64)
make build-all

# Build with podman
make build-all CONTAINER_ENGINE=podman

# Build for ARM64
make build-all PLATFORM=linux/arm64

# Build individual components
make build-frontend
make build-backend
make build-operator
make build-runner

# Push to registry
make push-all REGISTRY=quay.io/your-username
```

## Deployment

```bash
# Deploy with default images from quay.io/ambient_code
make deploy

# Deploy to custom namespace
make deploy NAMESPACE=my-namespace

# Deploy with custom images
cd components/manifests
cp env.example .env
# Edit .env with ANTHROPIC_API_KEY and CONTAINER_REGISTRY
./deploy.sh

# Clean up deployment
make clean
```

## Component Development

See component-specific documentation for detailed development commands:

- **Backend** (`components/backend/README.md`): Go API development, testing, linting
- **Frontend** (`components/frontend/README.md`): NextJS development, see also `DESIGN_GUIDELINES.md`
- **Operator** (`components/operator/README.md`): Operator development, watch patterns
- **Claude Code Runner** (`components/runners/claude-code-runner/README.md`): Python runner development

## Documentation

```bash
# Install documentation dependencies
pip install -r requirements-docs.txt

# Serve locally at http://127.0.0.1:8000
mkdocs serve

# Build static site
mkdocs build

# Deploy to GitHub Pages
mkdocs gh-deploy

# Markdown linting
markdownlint docs/**/*.md
```

## Local Development Helpers

```bash
# View logs
make dev-logs              # Both backend and frontend
make dev-logs-backend      # Backend only
make dev-logs-frontend     # Frontend only
make dev-logs-operator     # Operator only

# Operator management
make dev-restart-operator  # Restart operator deployment
make dev-operator-status   # Show operator status and events

# Cleanup
make dev-stop              # Stop processes, keep CRC running
make dev-stop-cluster      # Stop processes and shutdown CRC
make dev-clean             # Stop and delete OpenShift project

# Testing
make dev-test              # Run smoke tests
make dev-test-operator     # Test operator only
```

## Linting & Formatting

### Go (Backend & Operator)

```bash
# Check formatting (should output nothing)
cd components/backend && gofmt -l .
cd components/operator && gofmt -l .

# Detect suspicious constructs
go vet ./...

# Comprehensive linting
golangci-lint run

# Auto-format
gofmt -w components/backend components/operator
```

### Frontend

```bash
cd components/frontend
npm run build   # Build check (0 errors, 0 warnings)
npm run lint    # ESLint
```

### Python

```bash
# Formatting: black (double quotes)
# Import sorting: isort with black profile
# Linting: flake8 (ignore E203, W503)
```
