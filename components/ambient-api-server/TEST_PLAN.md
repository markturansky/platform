# Test Plan: Phase 1 — Core Data Layer

> Per-WP acceptance criteria with concrete test commands, expected results, and rollback steps.
> Companion to [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md).

---

## Scope and Philosophy

**This test plan validates that the API server behaves exactly as [`ambient-data-model.md`](ambient-data-model.md) and [`ambient-data-model.sql`](ambient-data-model.sql) specify.**

The API server is the contract enforcement layer. Its tests prove:

1. **Entity relationships** — every FK defined in the data model is enforced (SET NULL, CASCADE)
2. **Entity structure** — every Kind has the fields, types, and constraints the model specifies
3. **Business rules** — entity patterns ("AS agent WITH skills DO tasks"), junction table ordering, state machines
4. **CRUD behavior** — create, read, update, delete for all 8 Kinds + new Kinds (Project, ProjectSettings)
5. **Referential integrity** — no orphans, unique constraints, cascade deletes

**What is NOT in scope:**
- End-to-end testing (SDK owns this)
- Backend replacement validation (BE owns the behavioral reference; API implements equivalents)
- Control-plane reconciliation (CP owns informer/reconciler tests)
- Live OC deployment testing (separate operational concern)

### Data Model Contract

Source of truth: `ambient-data-model.md` (conceptual) + `ambient-data-model.sql` (technical schema).

The SQL schema defines the following contracts that tests MUST validate:

| Contract | SQL Source | Test Category |
|----------|-----------|---------------|
| 8 FK constraints (SET NULL + CASCADE) | Lines 74, 91-93, 105-106, 119-120 | FK validation tests |
| 2 UNIQUE constraints (junction tables) | Lines 107, 121 | Uniqueness tests |
| 16 indexes (FK columns + name + position) | Lines 125-143 | Index existence tests |
| CHECK constraints (`TRIM(name) != ''`) | Lines 24, 36, 48, 60, 73, 90 | Input validation tests |
| Session status enum (`active\|paused\|completed\|archived\|failed`) | Line 83 | State machine tests |
| Standardized entity pattern (id, name, repo_url, prompt) | Lines 16-97 | CRUD round-trip tests |
| Junction table ordering (position field) | Lines 101, 115 | Ordering tests |

---

## Test Infrastructure

### How Tests Run

```bash
make test    # OCM_ENV=integration_testing go test -p 1 -v ./...
```

- `-p 1` forces serial package execution (each package starts its own testcontainer PostgreSQL + API server)
- Each test function calls `test.RegisterIntegration(t)` which:
  1. Registers gomega with the test
  2. Truncates all tables via `helper.DBFactory.ResetDB()` (clean slate per test)
  3. Returns `(helper, openapi.APIClient)` pointing at the ephemeral API server
- Data seeded via service layer factories (bypasses HTTP auth)
- HTTP assertions via generated OpenAPI client with JWT authentication
- Testcontainer uses `postgres:14.2`, JWK mock validates RS256 tokens

### Standard 5-Test Pattern (per Kind)

Every plugin has 3 test files (`testmain_test.go`, `factory_test.go`, `integration_test.go`) with 5 tests:

| Test | Asserts |
|------|---------|
| `TestXxxGet` | 401 unauthenticated, 404 missing, 200 with id/kind/href/timestamps |
| `TestXxxPost` | 201 with id/kind/href assigned, 400 malformed JSON |
| `TestXxxPatch` | 200 preserves id/createdAt, 400 malformed JSON |
| `TestXxxPaging` | 20 records: page 1 = all 20, page 2 size 5 = 5 items with total=20 |
| `TestXxxListSearch` | TSL `id in ('...')` returns exactly 1 match |

### Baseline: Current Test State

Before any WP work begins, capture the current test baseline:

```bash
cd /home/mturansk/projects/src/github.com/ambient/platform/components/ambient-api-server
make test 2>&1 | tee /tmp/test_baseline_$(date +%Y%m%d_%H%M%S).log
echo $?
```

**Expected**: All 40 tests pass (5 tests × 8 plugins), exit code 0. If any test fails in baseline, that is a pre-existing issue and must be documented before proceeding.

---

## WP0 — Migration Hardening

### Risk Assessment

**HIGH RISK.** Adding FK constraints to a live database can fail if orphaned references exist (e.g., `sessions.workflow_id` pointing to a deleted or non-existent workflow). The current database has been running without referential integrity since creation.

### Pre-Migration Data Audit

Before applying the migration, run these queries against the live database to detect orphan references:

```sql
-- Orphan workflow.agent_id
SELECT w.id, w.agent_id FROM workflows w
LEFT JOIN agents a ON w.agent_id = a.id
WHERE w.agent_id IS NOT NULL AND a.id IS NULL AND w.deleted_at IS NULL;

-- Orphan session.created_by_user_id
SELECT s.id, s.created_by_user_id FROM sessions s
LEFT JOIN users u ON s.created_by_user_id = u.id
WHERE s.created_by_user_id IS NOT NULL AND u.id IS NULL AND s.deleted_at IS NULL;

-- Orphan session.assigned_user_id
SELECT s.id, s.assigned_user_id FROM sessions s
LEFT JOIN users u ON s.assigned_user_id = u.id
WHERE s.assigned_user_id IS NOT NULL AND u.id IS NULL AND s.deleted_at IS NULL;

-- Orphan session.workflow_id
SELECT s.id, s.workflow_id FROM sessions s
LEFT JOIN workflows w ON s.workflow_id = w.id
WHERE s.workflow_id IS NOT NULL AND w.id IS NULL AND s.deleted_at IS NULL;

-- Orphan workflow_skills.workflow_id
SELECT ws.id, ws.workflow_id FROM workflow_skills ws
LEFT JOIN workflows w ON ws.workflow_id = w.id
WHERE w.id IS NULL AND ws.deleted_at IS NULL;

-- Orphan workflow_skills.skill_id
SELECT ws.id, ws.skill_id FROM workflow_skills ws
LEFT JOIN skills s ON ws.skill_id = s.id
WHERE s.id IS NULL AND ws.deleted_at IS NULL;

-- Orphan workflow_tasks.workflow_id
SELECT wt.id, wt.workflow_id FROM workflow_tasks wt
LEFT JOIN workflows w ON wt.workflow_id = w.id
WHERE w.id IS NULL AND wt.deleted_at IS NULL;

-- Orphan workflow_tasks.task_id
SELECT wt.id, wt.task_id FROM workflow_tasks wt
LEFT JOIN tasks t ON wt.task_id = t.id
WHERE t.id IS NULL AND wt.deleted_at IS NULL;

-- Duplicate (workflow_id, skill_id) pairs
SELECT workflow_id, skill_id, COUNT(*) FROM workflow_skills
WHERE deleted_at IS NULL GROUP BY workflow_id, skill_id HAVING COUNT(*) > 1;

-- Duplicate (workflow_id, task_id) pairs
SELECT workflow_id, task_id, COUNT(*) FROM workflow_tasks
WHERE deleted_at IS NULL GROUP BY workflow_id, task_id HAVING COUNT(*) > 1;
```

**Action on orphans**: The migration must SET NULL on orphaned FK references before adding constraints. Duplicates must be deduplicated (keep lowest position, delete others).

### Migration Structure

Migration ID: `202602150001`. Must include:

1. **Orphan cleanup** — `UPDATE ... SET <fk_col> = NULL WHERE ...` for each FK column with orphans
2. **Deduplication** — Delete duplicate `(workflow_id, skill_id)` and `(workflow_id, task_id)` pairs
3. **FK constraints** — via raw `ALTER TABLE` SQL (not `db.CreateFK` which hardcodes `ON DELETE RESTRICT`)
4. **Unique constraints** — `(workflow_id, skill_id)` and `(workflow_id, task_id)`
5. **Indexes** — on all FK columns and `name` columns

### Rollback Migration

The rollback function must drop all constraints and indexes added by the migration:

```sql
ALTER TABLE workflows DROP CONSTRAINT IF EXISTS fk_workflows_agent_id;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS fk_sessions_created_by_user_id;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS fk_sessions_assigned_user_id;
ALTER TABLE sessions DROP CONSTRAINT IF EXISTS fk_sessions_workflow_id;
ALTER TABLE workflow_skills DROP CONSTRAINT IF EXISTS fk_workflow_skills_workflow_id;
ALTER TABLE workflow_skills DROP CONSTRAINT IF EXISTS fk_workflow_skills_skill_id;
ALTER TABLE workflow_skills DROP CONSTRAINT IF EXISTS uq_workflow_skills_workflow_skill;
ALTER TABLE workflow_tasks DROP CONSTRAINT IF EXISTS fk_workflow_tasks_workflow_id;
ALTER TABLE workflow_tasks DROP CONSTRAINT IF EXISTS fk_workflow_tasks_task_id;
ALTER TABLE workflow_tasks DROP CONSTRAINT IF EXISTS uq_workflow_tasks_workflow_task;
-- Drop all indexes added
```

### Test Commands

```bash
# Run all existing tests — constraints must not break them
make test

# Run only session tests (most FK-heavy)
OCM_ENV=integration_testing go test -p 1 -v ./plugins/sessions/...

# Run cross-kind integration test (exercises FK relationships)
OCM_ENV=integration_testing go test -p 1 -v ./test/integration/...
```

### Acceptance Criteria

- [ ] `make test` passes — all 40 existing tests work with constraints enabled
- [ ] New test: insert Session with non-existent `workflow_id` → returns error (not silent orphan)
- [ ] New test: insert Session with valid `workflow_id` → succeeds
- [ ] New test: delete Workflow referenced by Session → Session's `workflow_id` becomes NULL (ON DELETE SET NULL)
- [ ] New test: delete Workflow with WorkflowSkills → WorkflowSkills cascade-deleted (ON DELETE CASCADE)
- [ ] New test: insert duplicate `(workflow_id, skill_id)` pair → unique constraint violation
- [ ] New test: verify indexes exist via `SELECT indexname FROM pg_indexes WHERE tablename = 'sessions'`
- [ ] Rollback migration drops all constraints cleanly — `make test` still passes after rollback
- [ ] Data audit queries return zero orphans after migration runs

### New Test File

`plugins/sessions/constraints_test.go` — FK constraint validation tests (separate from standard 5-test pattern):

```
TestSessionFKWorkflowValid        — create workflow, create session with workflow_id → 201
TestSessionFKWorkflowInvalid      — create session with non-existent workflow_id → error
TestSessionFKCascadeSetNull       — delete workflow → session.workflow_id becomes NULL
TestWorkflowSkillFKCascade        — delete workflow → workflow_skills cascade-deleted
TestWorkflowSkillUniqueConstraint — duplicate (workflow_id, skill_id) → unique violation
TestIndexesExist                  — query pg_indexes, verify FK columns are indexed
```

### Dry-Run Procedure (Live Database)

Before merging WP0:

1. Take a database backup: `pg_dump -Fc ambient_api_server > /tmp/pre_wp0_backup.dump`
2. Restore to a test database: `pg_restore -d ambient_api_server_test /tmp/pre_wp0_backup.dump`
3. Run data audit queries against the test copy
4. Apply migration against the test copy
5. Run `make test` against the test copy (via `DB_FACTORY_MODE=external`)
6. Verify no data loss: `SELECT COUNT(*) FROM <each table>` before and after

---

## WP1 — Project Plugin

### Test Commands

```bash
# Run project plugin tests only
OCM_ENV=integration_testing go test -p 1 -v ./plugins/projects/...

# Run all tests (projects + existing)
make test
```

### Acceptance Criteria

- [ ] Standard 5-test pattern passes:
  - `TestProjectGet` — 401, 404, 200 with id/kind="Project"/href="/api/ambient-api-server/v1/projects/{id}"
  - `TestProjectPost` — 201 with id assigned, 400 malformed JSON
  - `TestProjectPatch` — 200 preserves id/createdAt, 400 malformed JSON
  - `TestProjectPaging` — 20 projects, page 1 = all, page 2 size 5 = 5
  - `TestProjectListSearch` — TSL `id in ('...')` returns 1 match
- [ ] New test: `TestProjectNameUnique` — POST two projects with same `name` → second returns 409 Conflict
- [ ] New test: `TestProjectSoftDelete` — DELETE → 200, subsequent GET → 404
- [ ] All 40 existing tests still pass
- [ ] `make test` passes (exit code 0)

### Test Files

```
plugins/projects/testmain_test.go     — TestMain bootstrap (identical pattern)
plugins/projects/factory_test.go      — newProject(), newProjectList(), stringPtr()
plugins/projects/integration_test.go  — 5 standard tests + TestProjectNameUnique + TestProjectSoftDelete
```

### Rollback

Migration rollback drops the `projects` table. Remove `_ "...plugins/projects"` from `main.go`. Revert OpenAPI changes.

---

## WP2 — ProjectSettings Plugin

### Test Commands

```bash
OCM_ENV=integration_testing go test -p 1 -v ./plugins/projectSettings/...
make test
```

### Acceptance Criteria

- [ ] Standard 5-test pattern passes (kind="ProjectSettings", path="/project_settings")
- [ ] New test: `TestProjectSettingsFKValid` — create Project, create ProjectSettings with its `project_id` → 201
- [ ] New test: `TestProjectSettingsFKInvalid` — create ProjectSettings with non-existent `project_id` → FK violation error
- [ ] New test: `TestProjectSettingsUniqueProjectId` — two ProjectSettings for same project → unique violation
- [ ] New test: `TestProjectSettingsCascadeDelete` — delete Project → ProjectSettings cascade-deleted
- [ ] New test: `TestProjectSettingsPatchJsonb` — PATCH `group_access` jsonb field without clobbering `runner_secrets`
- [ ] All existing tests still pass
- [ ] `make test` passes

### Test Files

```
plugins/projectSettings/testmain_test.go
plugins/projectSettings/factory_test.go     — newProjectSettings() creates a Project first, then settings
plugins/projectSettings/integration_test.go — 5 standard + 5 FK/unique/cascade/jsonb tests
```

### Rollback

Migration rollback drops `project_settings` table. Remove import from `main.go`.

---

## WP3 — Add Fields to Existing Kinds

### Test Commands

```bash
# Test each modified plugin individually
OCM_ENV=integration_testing go test -p 1 -v ./plugins/agents/...
OCM_ENV=integration_testing go test -p 1 -v ./plugins/skills/...
OCM_ENV=integration_testing go test -p 1 -v ./plugins/tasks/...
OCM_ENV=integration_testing go test -p 1 -v ./plugins/workflows/...
OCM_ENV=integration_testing go test -p 1 -v ./plugins/users/...

# Full suite
make test
```

### Acceptance Criteria

- [ ] Existing 5-test patterns still pass for all 5 modified plugins (backward compatible — new fields nullable)
- [ ] New test per plugin: POST with `project_id` → round-trips in GET response
- [ ] New test per plugin: PATCH `project_id` → updates correctly
- [ ] New test: POST Workflow with `branch` and `path` → round-trips
- [ ] New test: POST User with `groups` (JSON array) → round-trips
- [ ] New test: POST Agent with non-existent `project_id` → FK violation (if Project plugin is deployed)
- [ ] `make test` passes

### Modified Test Files

Update existing `factory_test.go` files to optionally include `project_id`. Add new test functions to existing `integration_test.go` files:

```
TestAgentProjectId        — POST/GET round-trip with project_id
TestAgentPatchProjectId   — PATCH project_id
TestWorkflowBranchPath    — POST with branch + path, verify GET
TestUserGroups            — POST with groups jsonb, verify GET
```

### Rollback

Each migration's rollback drops the added columns. No table drops — existing columns preserved.

---

## WP4 — Session Schema Expansion

### Test Commands

```bash
OCM_ENV=integration_testing go test -p 1 -v ./plugins/sessions/...
make test
```

### Acceptance Criteria

- [ ] Existing 5 session tests pass (all new fields nullable = backward compatible)
- [ ] New test: `TestSessionDataFieldsRoundTrip` — POST with `repos`, `interactive`, `timeout`, `llm_model`, `llm_temperature`, `llm_max_tokens`, `bot_account_name`, `resource_overrides`, `environment_variables`, `labels`, `annotations`, `project_id` → all returned in GET
- [ ] New test: `TestSessionStatusFieldsReadOnly` — status fields (`phase`, `start_time`, `completion_time`, `sdk_session_id`, etc.) returned in GET as null initially, NOT accepted in regular PATCH
- [ ] New test: `TestSessionKubeCrNameAutoSet` — POST → `kube_cr_name` equals `session.id` (KSUID), auto-set by BeforeCreate hook
- [ ] New test: `TestSessionParentSessionFK` — create parent session, create child with `parent_session_id` → round-trips. Non-existent parent → FK violation
- [ ] New test: `TestSessionProjectIdFK` — create Project, create Session with `project_id` → round-trips. Non-existent project → FK violation
- [ ] New test: `TestSessionPatchDataFields` — PATCH `llm_model`, `timeout` → updated. PATCH `phase` → ignored/rejected
- [ ] New test: `TestSessionReposJsonb` — POST with `repos` as JSON array of repo objects → round-trips correctly
- [ ] `make test` passes

### Modified Test Files

Update `plugins/sessions/factory_test.go` — add optional new fields to factory. Add new test functions to `plugins/sessions/integration_test.go`.

### Rollback

Migration rollback drops all new columns. `kube_cr_name`, status fields, data fields all removed. Existing 6 columns preserved.

---

## WP5 — Session Status Write-back Endpoint

### Test Commands

```bash
OCM_ENV=integration_testing go test -p 1 -v ./plugins/sessions/...
make test
```

### Acceptance Criteria

- [ ] New test: `TestSessionStatusPatch` — PATCH `/sessions/{id}/status` with `{"phase":"Running"}` → 200, GET shows `phase=Running`
- [ ] New test: `TestSessionStatusPatchMultipleFields` — PATCH with `phase`, `start_time`, `sdk_session_id`, `conditions` → all updated
- [ ] New test: `TestSessionStatusPatchPreservesData` — PATCH status → data fields (`name`, `prompt`, etc.) unchanged
- [ ] New test: `TestSessionStatusPatchNotFound` — PATCH `/sessions/nonexistent/status` → 404
- [ ] New test: `TestSessionRegularPatchIgnoresStatus` — regular PATCH `/sessions/{id}` with `phase` in body → `phase` NOT updated
- [ ] New test: `TestSessionStatusPatchAuth` — unauthenticated → 401
- [ ] Existing session tests pass
- [ ] `make test` passes

### New Route

`PATCH /api/ambient-api-server/v1/sessions/{id}/status` — registered in `plugins/sessions/plugin.go` alongside standard CRUD routes.

### Rollback

Remove the `/status` route registration, handler method, and service method. No schema changes.

---

## WP6 — Session Start/Stop Actions

### Test Commands

```bash
OCM_ENV=integration_testing go test -p 1 -v ./plugins/sessions/...
make test
```

### Acceptance Criteria

- [ ] New test: `TestSessionStart` — POST `/sessions/{id}/start` on new session → 200, `phase=Pending`
- [ ] New test: `TestSessionStop` — set `phase=Running` via status PATCH, POST `/sessions/{id}/stop` → 200, `phase=Stopping`
- [ ] New test: `TestSessionStartAlreadyRunning` — set `phase=Running`, POST start → 409 Conflict
- [ ] New test: `TestSessionStopAlreadyStopped` — set `phase=Stopped`, POST stop → 409 Conflict
- [ ] New test: `TestSessionStartNotFound` — POST `/sessions/nonexistent/start` → 404
- [ ] New test: `TestSessionStartAuth` — unauthenticated → 401
- [ ] New test: `TestSessionLifecycle` — create → start (Pending) → status patch Running → stop (Stopping) → status patch Stopped → start again (Pending)
- [ ] Existing session tests pass
- [ ] `make test` passes

### Phase State Machine

```
nil/empty → start → Pending
Stopped   → start → Pending
Pending   → (CP sets Running via status patch)
Running   → stop  → Stopping
Creating  → stop  → Stopping
Stopping  → (CP sets Stopped via status patch)
```

Invalid transitions return 409 with reason in error body.

### Rollback

Remove action route registrations, handler methods, service methods. No schema changes.

---

## WP7 — OpenAPI Spec Update + Client Regeneration

### Test Commands

```bash
# Regenerate client
make generate

# Verify compilation
go build ./...

# Verify all tests pass with regenerated client
make test

# Validate OpenAPI spec (if spectral or swagger-cli available)
# npx @stoplight/spectral-cli lint openapi/openapi.yaml
```

### Acceptance Criteria

- [ ] `make generate` succeeds without errors
- [ ] `go build ./...` succeeds with regenerated client
- [ ] All integration tests pass with the regenerated client types
- [ ] OpenAPI spec has no broken `$ref` references
- [ ] New schemas present: `Project`, `ProjectList`, `ProjectPatchRequest`, `ProjectSettings`, `ProjectSettingsList`, `ProjectSettingsPatchRequest`
- [ ] Session schema includes all data fields and readOnly status fields
- [ ] Status fields on Session marked `readOnly: true` in spec
- [ ] `SessionStatusPatchRequest` schema exists
- [ ] `/sessions/{id}/status`, `/sessions/{id}/start`, `/sessions/{id}/stop` paths exist
- [ ] Existing resource schemas include `project_id` field
- [ ] Workflow schema includes `branch` and `path` fields
- [ ] User schema includes `groups` field
- [ ] `make test` passes

### Rollback

Revert OpenAPI YAML changes. Re-run `make generate` to regenerate client from reverted spec.

---

## Cross-Kind Integration Tests

### Test Commands

```bash
OCM_ENV=integration_testing go test -p 1 -v ./test/integration/...
```

### New Cross-Kind Tests

Add to `test/integration/`:

```
TestProjectScopedWorkflow       — create Project, create Workflow with project_id, verify association
TestSessionLifecycleEndToEnd    — create Project → Workflow → Session → start → status patch Running → stop
TestCascadeDeleteProject        — delete Project → ProjectSettings cascade-deleted, Agent/Skill/Task/Workflow project_id SET NULL
TestFKIntegrityAcrossKinds      — verify all FK relationships hold across the full "AS agent WITH skills DO tasks" pattern
```

---

## CI Pipeline Requirements

### rh-trex-ai Dependency

The test suite requires the upstream framework at the pinned commit:

```bash
# Clone rh-trex-ai at the pinned commit before running tests
git clone https://github.com/openshift-online/rh-trex-ai.git \
  /home/mturansk/projects/src/github.com/openshift-online/rh-trex-ai
cd /home/mturansk/projects/src/github.com/openshift-online/rh-trex-ai
git checkout 04bc1dd
```

The `go.mod` replace directive points to this local path. CI must replicate this layout.

### Container Runtime

Tests require Podman or Docker for testcontainers:

```bash
# Podman
systemctl --user start podman.socket
export DOCKER_HOST=unix:///run/user/1000/podman/podman.sock

# Verify
podman ps
```

### Full CI Test Sequence

```bash
# 1. Clone rh-trex-ai at pinned commit
# 2. Start container runtime
# 3. Run tests
cd /home/mturansk/projects/src/github.com/ambient/platform/components/ambient-api-server
make test
echo "Exit code: $?"
# 4. Expected: exit code 0, all tests pass
```

---

## Definition of Done (per WP)

Each WP is complete when ALL of the following are true:

1. [ ] All new tests pass
2. [ ] All existing tests pass (`make test` exit code 0)
3. [ ] `go fmt ./...` produces no changes
4. [ ] `go build ./...` succeeds
5. [ ] Migration has a working rollback function
6. [ ] Rollback migration tested: apply → rollback → `make test` still passes
7. [ ] No secrets, tokens, or credentials in committed code
8. [ ] Code reviewed by human operator

---

## Appendix: Test Count Projection

| Phase | WP | New Tests | Cumulative Total |
|-------|-----|-----------|-----------------|
| Baseline | — | 0 | 40 (5 × 8 kinds) |
| 1a | WP0 | 6 (constraint tests) | 46 |
| 1a | WP1 | 7 (5 standard + 2 unique/delete) | 53 |
| 1a | WP2 | 10 (5 standard + 5 FK/unique/cascade/jsonb) | 63 |
| 1b | WP3 | 5 (project_id + branch/path + groups) | 68 |
| 1b | WP4 | 7 (data round-trip + status readOnly + kube_cr_name + parent FK + project FK + patch + repos) | 75 |
| 1c | WP5 | 6 (status patch + multi-field + preserves data + 404 + ignores regular + auth) | 81 |
| 1c | WP6 | 7 (start + stop + already running + already stopped + 404 + auth + lifecycle) | 88 |
| 1d | WP7 | 0 (covered by existing + new tests) | 88 |
| — | Cross-kind | 4 | 92 |
| — | Data model contract | 33 | 125 |

**Phase 1 target: 125 tests, all passing.**

---

## Data Model Contract Validation Tests

These tests exist independently of work packages. They validate the API behaves exactly as `ambient-data-model.md` and `ambient-data-model.sql` specify. Added in WP0, extended in each subsequent WP.

### Test File: `test/integration/data_model_contract_test.go`

#### Entity Structure Tests (Standardized Pattern)

Every Kind follows the pattern: `id`, `name`, `repo_url`, `prompt` + kind-specific fields.

```
TestEntityPattern_User          — POST with name → GET returns id/name/created_at/updated_at
TestEntityPattern_Agent         — POST with name, repo_url, prompt → all round-trip
TestEntityPattern_Skill         — POST with name, repo_url, prompt → all round-trip
TestEntityPattern_Task          — POST with name, repo_url, prompt → all round-trip
TestEntityPattern_Workflow      — POST with name, repo_url, prompt, agent_id → all round-trip
TestEntityPattern_Session       — POST with name, repo_url, prompt, created_by_user_id, assigned_user_id, workflow_id → all round-trip
TestEntityPattern_WorkflowSkill — POST with workflow_id, skill_id, position → all round-trip
TestEntityPattern_WorkflowTask  — POST with workflow_id, task_id, position → all round-trip
```

#### FK Constraint Tests (8 FKs from SQL schema)

```
TestFK_Workflow_AgentId_SetNull         — delete Agent → Workflow.agent_id becomes NULL
TestFK_Session_CreatedByUserId_SetNull  — delete User → Session.created_by_user_id becomes NULL
TestFK_Session_AssignedUserId_SetNull   — delete User → Session.assigned_user_id becomes NULL
TestFK_Session_WorkflowId_SetNull       — delete Workflow → Session.workflow_id becomes NULL
TestFK_WorkflowSkill_WorkflowId_Cascade — delete Workflow → WorkflowSkills cascade-deleted
TestFK_WorkflowSkill_SkillId_Cascade    — delete Skill → WorkflowSkills cascade-deleted
TestFK_WorkflowTask_WorkflowId_Cascade  — delete Workflow → WorkflowTasks cascade-deleted
TestFK_WorkflowTask_TaskId_Cascade      — delete Task → WorkflowTasks cascade-deleted
```

#### FK Violation Tests (invalid references rejected)

```
TestFK_Workflow_InvalidAgentId          — POST Workflow with non-existent agent_id → error
TestFK_Session_InvalidCreatedByUserId   — POST Session with non-existent created_by_user_id → error
TestFK_Session_InvalidWorkflowId        — POST Session with non-existent workflow_id → error
TestFK_WorkflowSkill_InvalidWorkflowId  — POST WorkflowSkill with non-existent workflow_id → error
TestFK_WorkflowSkill_InvalidSkillId     — POST WorkflowSkill with non-existent skill_id → error
TestFK_WorkflowTask_InvalidWorkflowId   — POST WorkflowTask with non-existent workflow_id → error
TestFK_WorkflowTask_InvalidTaskId       — POST WorkflowTask with non-existent task_id → error
```

#### Unique Constraint Tests

```
TestUnique_WorkflowSkill — POST duplicate (workflow_id, skill_id) pair → unique violation
TestUnique_WorkflowTask  — POST duplicate (workflow_id, task_id) pair → unique violation
TestUnique_AgentName     — POST two Agents with same name → 409 Conflict (per SQL UNIQUE on agents.name)
TestUnique_SkillName     — POST two Skills with same name → 409 Conflict (per SQL UNIQUE on skills.name)
```

#### Business Rule Tests (from ambient-data-model.md)

```
TestWorkflowComposition_AsAgentWithSkillsDoTasks — create Agent, 3 Skills, 2 Tasks, Workflow linking them, verify:
  - Workflow.agent_id points to Agent
  - 3 WorkflowSkills with positions 1,2,3
  - 2 WorkflowTasks with positions 1,2
  - GET Workflow returns correct agent_id
  - List WorkflowSkills filtered by workflow_id returns 3, ordered by position
  - List WorkflowTasks filtered by workflow_id returns 2, ordered by position

TestSessionExecution_WorkflowInstantiation — create Workflow, create Session with workflow_id, verify:
  - Session.workflow_id matches
  - Session.created_by_user_id matches creating user
  - Session has name, repo_url, prompt from creation

TestHumanAICollaboration_UserAssignment — create Session with created_by_user_id, PATCH assigned_user_id, verify:
  - created_by_user_id unchanged
  - assigned_user_id updated
  - Both point to valid Users

TestJunctionTableOrdering — create WorkflowSkills with positions [3, 1, 2], list with orderBy=position, verify:
  - Results come back in position order: 1, 2, 3

TestSessionStateMachine — verify Session status transitions per data model:
  - created → active → paused → active → completed
  - created → active → failed
  - created → active → archived
  - Invalid: completed → active (rejected)
```

#### Index Existence Tests

```
TestIndexes_AllFKColumnsIndexed — query pg_indexes for each table, verify:
  - workflows: idx on agent_id, name
  - sessions: idx on created_by_user_id, assigned_user_id, workflow_id, status
  - workflow_skills: idx on workflow_id, skill_id, (workflow_id, position)
  - workflow_tasks: idx on workflow_id, task_id, (workflow_id, position)
  - users: idx on name
  - agents: idx on name
  - skills: idx on name
  - tasks: idx on name
```

### Test Count: Data Model Contract

| Category | Tests |
|----------|-------|
| Entity structure | 8 |
| FK cascade/set-null | 8 |
| FK violation | 7 |
| Unique constraints | 4 |
| Business rules | 5 |
| Index existence | 1 (with sub-checks) |

**Total: 33 data model contract tests** (in addition to per-WP plugin tests)

---

## Backend Behavioral Reference

Per BE session's recommendation: backend test files serve as behavioral specifications for what "equivalent" means. API server tests should produce semantically equivalent behavior, not byte-identical responses.

| Backend Test File | Behaviors API Server Must Replicate |
|-------------------|-------------------------------------|
| `sessions_test.go` | Pagination, search filtering, unique name generation |
| `projects_test.go` | Name validation (lowercase + hyphens), namespace labels |
| `secrets_test.go` | Two-secret architecture, annotation filtering, key validation |
| `permissions_test.go` | RBAC role binding (view/edit/admin → ClusterRoles) |
| `content_test.go` | Path traversal prevention, utf8/base64 encoding |
| `middleware_test.go` | Auth header extraction (Bearer vs X-Forwarded-Access-Token precedence) |
| `github_auth_test.go` | HMAC state validation, OAuth callback flow |
| `repo_test.go` | SSAR-based access check (admin/edit/view role determination) |

Phase 1 covers Session CRUD, Project CRUD, and entity relationships. Secrets, permissions, content, auth flows are Phase 2+.

---

## Testing Responsibilities

| Component | Owns | Validates |
|-----------|------|-----------|
| **API Server** | Contract validation tests | Data model spec is enforced by the API |
| **SDK** | End-to-end tests | Assembled platform works (SDK → API → Postgres → CP → Operator) |
| **CP** | Informer/reconciler unit tests | Consumer correctly interprets API responses |
| **BE** | Behavioral reference catalog | Defines what "equivalent" means for replacement |
