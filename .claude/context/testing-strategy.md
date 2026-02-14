# Testing Strategy

**When to load:** Writing tests, running test suites, or setting up test infrastructure

## E2E Tests (Cypress - Portable)

**Purpose**: Automated end-to-end testing against any deployed instance.

**Location**: `e2e/`

**Quick Start**:

```bash
# Test against local kind cluster
make test-e2e-local

# Test against external cluster
export CYPRESS_BASE_URL=https://your-frontend.com
export TEST_TOKEN=$(oc whoami -t)
cd e2e && npm test
```

**Test Suites**:

- **vteam.cy.ts** (5 tests): Platform smoke tests — auth, workspace CRUD, API connectivity
- **sessions.cy.ts** (7 tests): Session management — creation, UI, workflows, agent interaction

**Total Runtime**: ~15 seconds (12 tests consolidated from original 29)

**What Gets Tested**:

- Workspace creation and navigation
- Session creation and UI components
- Workflow selection and cards
- Chat interface availability
- Breadcrumb navigation
- Backend API endpoints
- Real agent interaction (with ANTHROPIC_API_KEY)

**What Doesn't Get Tested**:

- OAuth proxy flow (uses direct token auth)
- OpenShift Routes (uses Ingress for kind)
- Long-running agent workflows (timeout constraints)
- Multi-user concurrent sessions

**CI Integration**: Tests run automatically on all PRs via GitHub Actions (`.github/workflows/e2e.yml`) using kind + Quay.io images.

**Local Development**:

```bash
# Kind with production images (Quay.io)
make kind-up        # Setup
make test-e2e       # Test
make kind-down      # Cleanup
```

**Key Features**:

- **Portable**: Tests run against any cluster (kind, CRC, dev, prod)
- **Fast**: 15-second runtime, one workspace reused across tests
- **Consolidated**: User journey tests, not isolated element checks
- **Real Agent Testing**: Verifies actual Claude responses (not hardcoded messages)

**Documentation**:
- [E2E Testing README](../../e2e/README.md)
- [Kind Local Dev Guide](../../docs/developer/local-development/kind.md)
- [E2E Testing Guide](../../docs/testing/e2e-guide.md)

## Backend Tests (Go)

- **Unit tests** (`tests/unit/`): Isolated component logic
- **Contract tests** (`tests/contract/`): API contract validation
- **Integration tests** (`tests/integration/`): End-to-end with real k8s cluster
  - Requires `TEST_NAMESPACE` environment variable
  - Set `CLEANUP_RESOURCES=true` for automatic cleanup
  - Permission tests validate RBAC boundaries

## Frontend Tests (NextJS)

- Jest for component testing (when configured)
- Cypress for e2e testing (see E2E Tests section above)

## Operator Tests (Go)

- Controller reconciliation logic tests
- CRD validation tests

## MkDocs Documentation

The MkDocs site (`mkdocs.yml`) provides:

- **User Guide**: Getting started, RFE creation, agent framework, configuration
- **Developer Guide**: Setup, architecture, plugin development, API reference, testing
- **Labs**: Hands-on exercises (basic, advanced, production)
- **Reference**: Agent personas, API endpoints, configuration schema, glossary

### Director Training Labs

Special lab track for leadership training located in `docs/labs/director-training/`:

- Structured exercises for understanding the vTeam system from a strategic perspective
- Validation reports for tracking completion and understanding
