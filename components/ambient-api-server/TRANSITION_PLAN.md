# TRANSITION_PLAN.md — Backend → API Server Migration Validation

**Author:** BE session
**Date:** 2026-02-14
**Status:** DRAFT — awaiting Overlord approval

---

## 1. Executive Summary

The legacy backend (75 routes, Gin, K8s CRDs) is being replaced by the API server (gorilla/mux, Postgres, rh-trex-ai framework). This plan defines how we prove the replacement works by adapting the backend's existing behavioral tests to run against the API server's endpoints.

**Principle:** Leave all existing backend tests intact. They are the working baseline. Create new, functionally equivalent tests in the API server's test infrastructure that prove the same behaviors exist in the replacement.

**What this document covers:**
- Complete 75-route inventory categorized by migration phase
- Behavioral mapping: backend test → API server equivalent test
- Phased execution plan with pass/fail gates
- Dual-run comparison strategy for the transition period
- Shutdown acceptance criteria

---

## 2. Route Inventory (75 routes, Normal Server Mode)

### 2.1 Phase 1 — Core CRUD (30 routes)

These routes have direct API server equivalents or will be created in upcoming WPs.

#### Session CRUD → API server `/v1/sessions`

| Backend Route | Method | API Server Equivalent | WP |
|---|---|---|---|
| `/api/projects/:projectName/agentic-sessions` | GET | `GET /v1/sessions` (with project filter) | WP3 |
| `/api/projects/:projectName/agentic-sessions` | POST | `POST /v1/sessions` | WP3 |
| `/api/projects/:projectName/agentic-sessions/:sessionName` | GET | `GET /v1/sessions/{id}` | WP3 |
| `/api/projects/:projectName/agentic-sessions/:sessionName` | PUT | `PATCH /v1/sessions/{id}` | WP3 |
| `/api/projects/:projectName/agentic-sessions/:sessionName` | PATCH | `PATCH /v1/sessions/{id}` | WP3 |
| `/api/projects/:projectName/agentic-sessions/:sessionName` | DELETE | `DELETE /v1/sessions/{id}` | WP3 |
| `/api/projects/:projectName/agentic-sessions/:sessionName/clone` | POST | New endpoint needed | WP3+ |
| `/api/projects/:projectName/agentic-sessions/:sessionName/start` | POST | CP handles via annotation | WP3+ |
| `/api/projects/:projectName/agentic-sessions/:sessionName/stop` | POST | CP handles via annotation | WP3+ |
| `/api/projects/:projectName/agentic-sessions/:sessionName/displayname` | PUT | `PATCH /v1/sessions/{id}` (display_name field) | WP3 |
| `/api/projects/:projectName/agentic-sessions/:sessionName/repos` | POST | Session field update | WP3+ |
| `/api/projects/:projectName/agentic-sessions/:sessionName/repos/status` | GET | Session field read | WP3+ |
| `/api/projects/:projectName/agentic-sessions/:sessionName/repos/:repoName` | DELETE | Session field update | WP3+ |

#### Project CRUD → API server `/v1/projects`

| Backend Route | Method | API Server Equivalent | WP |
|---|---|---|---|
| `/api/projects` | GET | `GET /v1/projects` | WP1 (DONE) |
| `/api/projects` | POST | `POST /v1/projects` | WP1 (DONE) |
| `/api/projects/:projectName` | GET | `GET /v1/projects/{id}` | WP1 (DONE) |
| `/api/projects/:projectName` | PUT | `PATCH /v1/projects/{id}` | WP1 (DONE) |
| `/api/projects/:projectName` | DELETE | `DELETE /v1/projects/{id}` | WP1 (DONE) |

#### Secrets Management (6 routes)

| Backend Route | Method | API Server Equivalent | WP |
|---|---|---|---|
| `/api/projects/:projectName/secrets` | GET | New plugin needed | WP4 |
| `/api/projects/:projectName/runner-secrets` | GET | New plugin needed | WP4 |
| `/api/projects/:projectName/runner-secrets` | PUT | New plugin needed | WP4 |
| `/api/projects/:projectName/integration-secrets` | GET | New plugin needed | WP4 |
| `/api/projects/:projectName/integration-secrets` | PUT | New plugin needed | WP4 |
| `/api/projects/:projectName/keys` | GET/POST/DELETE | New plugin needed | WP4 |

#### Permissions/RBAC (3 routes)

| Backend Route | Method | API Server Equivalent | WP |
|---|---|---|---|
| `/api/projects/:projectName/permissions` | GET | New plugin or middleware | WP5 |
| `/api/projects/:projectName/permissions` | POST | New plugin or middleware | WP5 |
| `/api/projects/:projectName/permissions/:subjectType/:subjectName` | DELETE | New plugin or middleware | WP5 |

### 2.2 Phase 2 — Deferred Routes (33 routes)

These routes are tightly coupled to runner pod filesystems, git operations, or AG-UI streaming. They stay on the backend during transition and migrate later.

#### Content/File Operations (7 routes — content service sidecar)

| Backend Route | Method | Notes |
|---|---|---|
| `/content/write` | POST | Runs inside runner pods only (`CONTENT_SERVICE_MODE=true`) |
| `/content/file` | GET | Runner pod local filesystem |
| `/content/list` | GET | Runner pod local filesystem |
| `/content/delete` | DELETE | Runner pod local filesystem |
| `/content/git-status` | GET | Runner pod git state |
| `/content/git-configure-remote` | POST | Runner pod git config |
| `/content/workflow-metadata` | GET | Runner pod workflow state |

**Decision:** These never migrate to API server. They remain on the content service sidecar inside runner pods. No behavioral tests needed in API server.

#### Session Workspace & Git (8 routes)

| Backend Route | Method | Notes |
|---|---|---|
| `.../workspace` | GET | Proxied to content service in runner pod |
| `.../workspace/*path` | GET/PUT/DELETE | File operations proxied to runner pod |
| `.../git/status` | GET | Proxied to runner pod |
| `.../git/configure-remote` | POST | Proxied to runner pod |
| `.../git/list-branches` | GET | Proxied to runner pod |

**Decision:** These proxy to runner pods. Keep on backend until AG-UI migration is complete.

#### AG-UI/SSE Protocol (6 routes)

| Backend Route | Method | Notes |
|---|---|---|
| `.../agui/run` | POST | SSE fan-out, 2,895 lines across 6 files |
| `.../agui/interrupt` | POST | Forwarded to runner pod |
| `.../agui/feedback` | POST | Forwarded to runner pod |
| `.../agui/events` | GET | SSE streaming with MessageCompactor |
| `.../agui/history` | GET | JSONL event persistence |
| `.../agui/runs` | GET | Run listing |
| `.../mcp/status` | GET | MCP tool status |
| `.../export` | GET | Session export |

**Decision:** Requires dedicated spike before migration. Keep on backend.

#### Repository Browsing (5 routes)

| Backend Route | Method | Notes |
|---|---|---|
| `.../users/forks` | GET/POST | GitHub API proxy |
| `.../repo/tree` | GET | GitHub/GitLab API proxy |
| `.../repo/blob` | GET | GitHub/GitLab API proxy |
| `.../repo/branches` | GET | GitHub/GitLab API proxy |
| `.../repo/seed-status` | GET | `.claude` directory detection |
| `.../repo/seed` | POST | Repository seeding |

**Decision:** Pure API proxying. Can migrate to API server in Phase 2. Low complexity.

### 2.3 Phase 3 — OAuth & Auth Flows (12 routes)

| Backend Route | Method | Notes |
|---|---|---|
| `/oauth2callback` | GET | HMAC-SHA256 signed state params |
| `/oauth2callback/status` | GET | Callback endpoint discovery |
| `/api/auth/github/*` | Various (8) | GitHub OAuth, PAT, installation linking |
| `/api/auth/google/*` | Various (3) | Google OAuth connect/status/disconnect |
| `/api/auth/gitlab/*` | Various (4) | GitLab connect/status/disconnect/test |
| `/api/auth/jira/*` | Various (4) | Jira connect/status/disconnect/test |
| `/api/auth/integrations/status` | GET | Aggregated integration status |
| `.../credentials/*` | GET (4) | Per-session credential retrieval |

**Decision:** OAuth callbacks have registered redirect URIs with external providers. During transition, backend continues handling OAuth in "OAuth-only proxy mode." Re-registration happens at final cutover.

### 2.4 Miscellaneous (5 routes)

| Backend Route | Method | API Server Equivalent |
|---|---|---|
| `/health` | GET | API server has its own health endpoint |
| `/api/cluster-info` | GET | New endpoint or removed |
| `.../access` | GET | SSAR-based, keep or replace with API server authz |
| `.../integration-status` | GET | Per-project integration status |
| `.../k8s-resources` | GET | K8s resource listing for session |
| `.../workflow` | POST | Workflow selection for session |
| `.../workflow/metadata` | GET | Workflow metadata retrieval |
| `/api/workflows/ootb` | GET | Out-of-the-box workflow listing |
| `.../oauth/:provider/url` | GET | OAuth URL generation |

---

## 3. Behavioral Test Adaptation Strategy

### 3.1 Principle

Every backend handler test file documents a **behavioral contract** — what the system does when given specific inputs. The API server must honor these contracts. We prove this by writing new tests in the API server's test infrastructure that assert the same behaviors.

**We do NOT:**
- Modify existing backend tests
- Port backend test infrastructure (Ginkgo/Gomega/fake K8s clients) to the API server
- Test K8s-specific mechanics (CRDs, SSAR, fake clients) in the API server — those are replaced by Postgres + JWT

**We DO:**
- Map each backend behavioral assertion to an API server integration test
- Use the API server's existing test patterns (`test.RegisterIntegration`, generated OpenAPI client, testcontainer Postgres)
- Add tests incrementally as each WP lands

### 3.2 Backend Test → API Server Test Mapping

#### `sessions_test.go` → `plugins/sessions/integration_test.go`

| Backend Behavior | Backend Assert | API Server Equivalent | Status |
|---|---|---|---|
| Empty list returns `{items: []}` | `Expect(items).To(HaveLen(0))` | `TestSessionGet` 404 case | EXISTS |
| List returns all sessions | `Expect(items).To(HaveLen(2))` | `TestSessionPaging` (create 20, list all) | EXISTS |
| Pagination (offset/limit) | `hasMore`, `totalCount` | `TestSessionPaging` (page/size) | EXISTS — different param names |
| Search/filter by name | `Expect(items).To(HaveLen(1))` | `TestSessionListSearch` (by id) | EXISTS — search by id, needs name search |
| Create session | `AssertHTTPStatus(http.StatusCreated)` | `TestSessionPost` | EXISTS |
| Get session by name | `AssertHTTPStatus(http.StatusOK)` | `TestSessionGet` | EXISTS |
| Update session | `AssertHTTPStatus(http.StatusOK)` | `TestSessionPatch` | EXISTS |
| Delete session | `AssertHTTPStatus(http.StatusNoContent)` | Needs `TestSessionDelete` | **NEW** |
| Unique name generation | custom logic test | Needs equivalent | **NEW** (WP3) |
| AutoPush parsing | field validation | Needs equivalent | **NEW** (WP3) |
| Auth required (401) | `AssertHTTPStatus(http.StatusUnauthorized)` | `TestSessionGet` unauthenticated case | EXISTS |

#### `projects_test.go` → `plugins/projects/integration_test.go`

| Backend Behavior | API Server Equivalent | Status |
|---|---|---|
| Create project with valid name | `TestProjectPost` | EXISTS |
| Reject invalid names (uppercase, underscore, spaces) | Needs name validation test | **NEW** — API server has no name validation yet |
| Reject empty name | `TestProjectPost` 400 case | EXISTS (malformed body) |
| List managed projects only | `TestProjectPaging` | EXISTS (no label filtering — Postgres replaces K8s label selectors) |
| Get project by name/id | `TestProjectGet` | EXISTS |
| Delete project | Needs `TestProjectDelete` | **NEW** |
| 404 for non-existent project | `TestProjectGet` 404 case | EXISTS |
| 409 for duplicate project | Needs duplicate test | **NEW** — unique index on `name` exists in model |
| Auth required | `TestProjectGet` 401 case | EXISTS |
| Namespace labels (managed-by) | N/A — Postgres replaces K8s namespaces | SKIP |

#### `secrets_test.go` → New `plugins/secrets/integration_test.go` (WP4)

| Backend Behavior | API Server Equivalent | Status |
|---|---|---|
| List runner secrets (annotation-filtered) | List secrets by type/category | **NEW** (WP4) |
| Create/update runner secrets | Create/update secrets | **NEW** (WP4) |
| Key validation (ANTHROPIC_API_KEY only for runner) | Input validation | **NEW** (WP4) |
| Two-secret architecture enforcement | Category-based separation | **NEW** (WP4) |
| Integration secrets (flexible keys) | Create/update with any keys | **NEW** (WP4) |
| Auth required | 401 case | **NEW** (WP4) |

#### `permissions_test.go` → New permissions mechanism (WP5)

| Backend Behavior | API Server Equivalent | Status |
|---|---|---|
| List role bindings | List project permissions | **NEW** (WP5) |
| Add permission (view/edit/admin) | Grant permission | **NEW** (WP5) |
| Remove permission | Revoke permission | **NEW** (WP5) |
| Reject invalid role names | Input validation | **NEW** (WP5) |
| SSAR-based access check | JWT + API server authz | **NEW** (WP5) |

#### `middleware_test.go` → API server auth middleware

| Backend Behavior | API Server Equivalent | Status |
|---|---|---|
| Project name validation (lowercase, hyphens, length) | Needs validation middleware | **NEW** — critical for parity |
| Bearer token extraction | JWT middleware in rh-trex-ai | EXISTS (framework-provided) |
| X-Forwarded-Access-Token fallback | Needs evaluation | **DECISION NEEDED** |
| Token precedence (Bearer > X-Forwarded) | Needs evaluation | **DECISION NEEDED** |

#### `content_test.go` → NOT MIGRATED

Content handler tests validate runner-pod filesystem operations (path traversal prevention, git push/pull, file read/write). These run inside the content service sidecar, not the API server. **No API server equivalent needed.**

#### `github_auth_test.go` / `gitlab_auth_test.go` → Phase 3

OAuth tests validate HMAC state signing, callback flows, token/URL validation. These stay on the backend during transition. API server equivalents written when OAuth migrates.

#### `repo_test.go` → Phase 2

Repository access tests validate SSAR-based permission checks and GitHub/GitLab API proxying. Stays on backend during Phase 1. Migrates when repo browsing moves to API server.

### 3.3 Tests That DON'T Need API Server Equivalents

| Backend Test File | Reason |
|---|---|
| `content_test.go` | Content service runs in runner pods, not API server |
| `repo_seed_test.go` | `.claude` directory detection — runner pod concern |
| `operations_test.go` | Git token retrieval — credential injection at pod level |
| `runtime_credentials_test.go` | Identity fetching — K8s service account concern |
| `display_name_test.go` | Covered by session PATCH (display_name is a field) |
| `common_test.go` | Provider detection — utility function, tested inline |
| `health_test.go` | API server has its own health endpoint |

---

## 4. Phased Execution Plan

### Phase 1a: Validate Existing API Server Tests (NOW)

**Owner:** API session
**Gate:** All existing integration tests pass (`make test` = 51/51)
**Status:** DONE (WP0 complete)

### Phase 1b: Project CRUD Parity Tests (WP1)

**Owner:** API session
**New tests to add to `plugins/projects/integration_test.go`:**

```
TestProjectDelete              — DELETE /v1/projects/{id}, verify 204 + gone
TestProjectDuplicateName       — POST with existing name, verify 409
TestProjectNameValidation      — POST with invalid names (uppercase, underscore, spaces, >63 chars), verify 400
TestProjectGetByName           — GET with search("name = 'xxx'"), verify single result
```

**Behavioral source:** `backend/handlers/projects_test.go` lines 88-138 (name validation), 319-345 (duplicate), 604-687 (delete)

**Pass gate:** 4 new tests pass. Existing 5 still pass. Total: 9 project tests.

### Phase 1c: Session CRUD Parity Tests (WP3)

**Owner:** API session
**New tests to add to `plugins/sessions/integration_test.go`:**

```
TestSessionDelete              — DELETE /v1/sessions/{id}, verify 204 + gone
TestSessionNameSearch          — GET with search("name like 'xxx%'"), verify results
TestSessionProjectFilter       — GET with X-Ambient-Project header, verify scoping
TestSessionClone               — POST /v1/sessions/{id}/clone (when endpoint exists)
TestSessionStartStop           — Session phase transitions (via status PATCH)
TestSessionAutoFields          — Verify auto-generated fields (kube_cr_name = KSUID)
```

**Behavioral source:** `backend/handlers/sessions_test.go` — pagination (lines ~170-200), search (lines ~210-260), CRUD lifecycle

**Pass gate:** 6 new tests pass. Existing 5 still pass. Total: 11 session tests.

### Phase 2: Secrets + Permissions + Repo Browsing (WP4-5)

**Owner:** API session (secrets/permissions plugins), BE advisory
**New test files:**

```
plugins/secrets/integration_test.go        — 8 tests (CRUD + key validation + two-secret arch)
plugins/permissions/integration_test.go    — 6 tests (grant/revoke/list + role validation)
```

**Behavioral source:** `backend/handlers/secrets_test.go`, `backend/handlers/permissions_test.go`

### Phase 3: OAuth Migration + AG-UI Spike

**Owner:** TBD
**Prerequisite:** AG-UI spike completed, OAuth redirect URIs re-registered
**New tests:** OAuth flow tests adapted from `github_auth_test.go`, `gitlab_auth_test.go`

---

## 5. Dual-Run Comparison (Transition Period)

During the strangler-fig transition, both systems serve traffic. A comparison script validates equivalence.

### 5.1 What to Compare

| Endpoint Category | Backend Path | API Server Path | Comparison |
|---|---|---|---|
| List sessions | `GET /api/projects/{p}/agentic-sessions` | `GET /v1/sessions?search=project_name='{p}'` | Same count, same names |
| Get session | `GET /api/projects/{p}/agentic-sessions/{n}` | `GET /v1/sessions/{id}` | Same fields (mapped) |
| List projects | `GET /api/projects` | `GET /v1/projects` | Same count, same names |
| Health | `GET /health` | API server health endpoint | Both 200 |

### 5.2 Field Mapping

The response shapes differ. The comparison script maps fields:

| Backend Field | API Server Field | Notes |
|---|---|---|
| `.metadata.name` (K8s) | `.id` (KSUID) | Different ID schemes |
| `.metadata.namespace` | `.project_name` (via JOIN) | Different scoping |
| `.metadata.creationTimestamp` | `.created_at` | Same semantics |
| `.spec.prompt` | `.prompt` | Direct mapping |
| `.spec.repos` | `.repo_url` | Array → single (WP3 decision) |
| `.status.phase` | `.status` | Direct mapping |

### 5.3 Comparison Cadence

- **Pre-cutover:** Run hourly. Any divergence blocks cutover.
- **During cutover:** Run every 5 minutes for first 4 hours.
- **Post-cutover:** Run daily for 1 week, then weekly for 1 month.

---

## 6. Backend Shutdown Acceptance

### 6.1 Pre-conditions

All must be true before scaling backend to 0:

- [ ] All Phase 1 API server tests pass (sessions + projects CRUD parity)
- [ ] Dual-run comparison shows zero divergence for 48 hours
- [ ] Frontend switched to API server endpoints (route change in ingress/proxy config)
- [ ] CP reconciler confirmed working against API server (not backend CRDs)
- [ ] SDK confirmed working against API server endpoints
- [ ] OAuth either migrated or backend kept alive in OAuth-only proxy mode

### 6.2 Shutdown Test

1. `oc scale deployment/backend --replicas=0`
2. Monitor 48 hours:
   - Frontend operations: session create/list/delete, project management
   - Operator: continues creating/managing pods via CP → API server → K8s
   - OAuth: works (if migrated) or proxy still handles (if kept)
   - Ingress logs: no 5xx errors
3. If any failure → `oc scale deployment/backend --replicas=1` → investigate

### 6.3 Pass Criteria

48 hours with backend scaled to 0, zero user-visible impact.

---

## 7. Rollback Strategy

### 7.1 Pre-Cutover Rollback (Low Risk)

Backend is still running. Just revert the ingress/proxy config to point back to backend endpoints. No data loss — both systems write to their respective stores.

### 7.2 Post-Cutover Rollback (Medium Risk)

Backend has been off for some time. Data created in API server (Postgres) has no corresponding CRDs.

**Rollback steps:**
1. Scale backend back to 1 replica
2. Revert ingress/proxy config
3. Data created during API-server-only period is lost from backend's perspective
4. CP reconciler must be reverted to watch CRDs instead of API server

**Mitigation:** Keep backend deployment at 0 replicas (not deleted) for 30 days after cutover. Delete only after 30-day soak.

---

## 8. Timeline

| Phase | Target | Gate |
|---|---|---|
| WP0 (Migration Hardening) | DONE | 51/51 tests pass |
| WP1 (Projects + ProjectSettings) | In progress | 9 project tests pass |
| CP Phase 0 (28 tests) | In progress | Tests pass |
| WP3 (Sessions) | After WP1 | 11 session tests pass |
| Dual-run comparison | After WP3 | Zero divergence 48h |
| WP4-5 (Secrets + Permissions) | After WP3 | 14 new tests pass |
| Backend shutdown test | After all WPs | 48h soak pass |
| Backend decommission | 30 days post-shutdown | No rollback needed |

---

## Appendix A: Backend Test File Reference

All files at `components/backend/handlers/`:

| File | Lines | Behaviors Documented | Migration Phase |
|---|---|---|---|
| `sessions_test.go` | ~800 | Pagination, search, CRUD, unique names, autoPush | Phase 1c |
| `projects_test.go` | 793 | Name validation, CRUD, labels, RBAC, duplicates | Phase 1b |
| `secrets_test.go` | 842 | Two-secret arch, key validation, annotation filter | Phase 2 |
| `permissions_test.go` | 880 | Role binding CRUD, SSAR checks, role validation | Phase 2 |
| `middleware_test.go` | 313 | Project validation, token extraction, RBAC | Phase 1b |
| `content_test.go` | ~500 | Path traversal, git ops, file encoding | NOT MIGRATED |
| `github_auth_test.go` | ~300 | HMAC state, OAuth callback | Phase 3 |
| `gitlab_auth_test.go` | ~200 | Token/URL validation | Phase 3 |
| `repo_test.go` | 785 | SSAR access, repo URL parsing, provider detection | Phase 2 |
| `repo_seed_test.go` | ~200 | .claude directory detection | NOT MIGRATED |

## Appendix B: Key Differences Between Test Infrastructures

| Aspect | Backend Tests | API Server Tests |
|---|---|---|
| Framework | Ginkgo v2 / Gomega (BDD) | Go `testing` / Gomega (standard) |
| HTTP framework | Gin (test context, no server) | gorilla/mux (real server on localhost:0) |
| Data store | Fake K8s clients (in-memory) | Testcontainer Postgres (real DB) |
| Auth | Fake token reviews + SSAR reactors | Mock JWK server + JWT |
| Route structure | `/api/projects/:projectName/...` | `/api/ambient-api-server/v1/...` |
| ID scheme | K8s name (string) | KSUID (api.NewID()) |
| Project scoping | URL path parameter | `X-Ambient-Project` header / search filter |
| Test client | Direct handler function call | Generated OpenAPI client |
| DB reset | Fresh fake client per test | `helper.DBFactory.ResetDB()` per test function |
