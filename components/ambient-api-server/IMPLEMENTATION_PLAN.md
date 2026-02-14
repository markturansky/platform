# Implementation Plan: Phase 1 — Core Data Layer

> **Prerequisite**: Read [DATA_MODEL_COMPARISON.md](DATA_MODEL_COMPARISON.md) for the architecture rationale, field inventory, and cross-session contracts.
>
> This document is the execution-level plan. It breaks Phase 1 into testable sub-phases with dependency order, files touched, and acceptance criteria a human can verify.

---

## Dependency Graph

```
Phase 1a: Foundation
  WP0 (Migration hardening) ──→ WP1 (Project plugin) ──→ WP2 (ProjectSettings plugin)
                                       ↓
Phase 1b: Schema Expansion
  WP3 (project_id on Agent/Skill/Task/Workflow + Workflow branch/path + User groups)
                                       ↓
  WP4 (Session schema expansion — 24 new fields)
                                       ↓
Phase 1c: Session Lifecycle
  WP5 (Session status write-back endpoint)
  WP6 (Session start/stop actions)
                                       ↓
Phase 1d: OpenAPI + Client Regen
  WP7 (OpenAPI spec update + client regeneration — runs incrementally after each WP)
```

Each sub-phase is independently deployable and testable. No sub-phase depends on a later one.

---

## Phase 1a: Foundation

### WP0 — Migration Hardening

**Why**: Overlord Concern 4. Current GORM migrations create bare tables with no FK constraints, no indexes, no unique constraints. The SQL schema (`ambient-data-model.sql`) defines 8 FKs, 4 unique constraints, 16 indexes, and CHECK constraints that the Go migrations ignore. New fields must not repeat this gap.

**Scope**: Add a new migration that applies FK constraints and indexes to all existing tables. This is a one-time catch-up migration that runs after the existing 8 migrations.

**Files**:
- `plugins/sessions/migration.go` — add `migration_constraints()` with FK + index DDL
- OR: a dedicated `pkg/db/migrations/` file if cross-plugin constraints are cleaner

**What to add** (via `tx.Exec()` raw SQL, not AutoMigrate):

| Table | Constraint | Type |
|-------|-----------|------|
| `workflows` | `agent_id` → `agents(id)` ON DELETE SET NULL | FK |
| `sessions` | `created_by_user_id` → `users(id)` ON DELETE SET NULL | FK |
| `sessions` | `assigned_user_id` → `users(id)` ON DELETE SET NULL | FK |
| `sessions` | `workflow_id` → `workflows(id)` ON DELETE SET NULL | FK |
| `workflow_skills` | `workflow_id` → `workflows(id)` ON DELETE CASCADE | FK |
| `workflow_skills` | `skill_id` → `skills(id)` ON DELETE CASCADE | FK |
| `workflow_skills` | `(workflow_id, skill_id)` UNIQUE | Unique |
| `workflow_tasks` | `workflow_id` → `workflows(id)` ON DELETE CASCADE | FK |
| `workflow_tasks` | `task_id` → `tasks(id)` ON DELETE CASCADE | FK |
| `workflow_tasks` | `(workflow_id, task_id)` UNIQUE | Unique |
| All tables | Indexes on FK columns, name columns | Index |

**Test criteria**:
- [ ] `make test` passes — existing tests work with constraints enabled
- [ ] Insert with invalid FK returns error (not silent orphan)
- [ ] Duplicate `(workflow_id, skill_id)` pair returns unique violation
- [ ] `\d sessions` in psql shows FK constraints on all 3 reference columns

**Migration ID**: `202602150001`

---

### WP1 — Project Plugin (new Kind)

**Scope**: Full CRUD plugin for Project. This is the multi-tenant anchor — all other Kinds will FK to it.

**Files created** (7 plugin files + 1 OpenAPI spec):
- `plugins/projects/plugin.go`
- `plugins/projects/model.go`
- `plugins/projects/handler.go`
- `plugins/projects/service.go`
- `plugins/projects/dao.go`
- `plugins/projects/presenter.go`
- `plugins/projects/migration.go`
- `plugins/projects/mock_dao.go`
- `openapi/openapi.projects.yaml`

**Files modified**:
- `cmd/ambient-api-server/main.go` — add `_ "...plugins/projects"` import
- `openapi/openapi.yaml` — add Project paths + schema refs

**Model** (`model.go`):
```
Project {
    api.Meta
    Name         string  `gorm:"uniqueIndex;not null"`
    DisplayName  *string
    Description  *string
    Labels       *datatypes.JSON
    Annotations  *datatypes.JSON
    Status       *string
}
```

**Migration** (`migration.go`):
- Table `projects` with `name` unique index
- Migration ID: `202602150010`

**Test criteria**:
- [ ] POST creates project, GET returns it with correct HATEOAS fields
- [ ] POST with duplicate `name` returns 409 Conflict
- [ ] List/search/paging work (standard 5-test pattern)
- [ ] DELETE soft-deletes, subsequent GET returns 404
- [ ] `make test` passes

**Test files** (3 files, following existing pattern):
- `plugins/projects/testmain_test.go`
- `plugins/projects/factory_test.go`
- `plugins/projects/integration_test.go`

---

### WP2 — ProjectSettings Plugin (new Kind)

**Scope**: Full CRUD plugin. One-to-one with Project via unique `project_id` FK.

**Files**: Same 7+1 pattern as WP1, in `plugins/projectSettings/`.

**Model** (`model.go`):
```
ProjectSettings {
    api.Meta
    ProjectId    string  `gorm:"uniqueIndex;not null"`
    GroupAccess   *datatypes.JSON
    RunnerSecrets *datatypes.JSON
    Repositories *datatypes.JSON
}
```

**Migration**:
- FK: `project_id` → `projects(id)` ON DELETE CASCADE
- Unique index on `project_id`
- Migration ID: `202602150020`

**Test criteria**:
- [ ] POST with valid `project_id` succeeds
- [ ] POST with non-existent `project_id` returns FK violation error
- [ ] POST second settings for same project returns unique violation
- [ ] PATCH updates individual jsonb fields without clobbering others
- [ ] Standard 5-test pattern passes
- [ ] `make test` passes

---

## Phase 1b: Schema Expansion

### WP3 — Add Fields to Existing Kinds

**Scope**: Add `project_id` FK to Agent, Skill, Task, Workflow. Add `branch` + `path` to Workflow. Add `groups` to User.

**Files modified per plugin** (4 plugins × 5 files + User):

| Plugin | model.go | migration.go | handler.go | presenter.go | OpenAPI YAML |
|--------|----------|-------------|------------|-------------|-------------|
| agents | +`ProjectId` | new migration | +patch field | +field mapping | +`project_id` |
| skills | +`ProjectId` | new migration | +patch field | +field mapping | +`project_id` |
| tasks | +`ProjectId` | new migration | +patch field | +field mapping | +`project_id` |
| workflows | +`ProjectId`, `Branch`, `Path` | new migration | +patch fields | +field mapping | +3 fields |
| users | +`Groups` | new migration | +patch field | +field mapping | +`groups` |

**Migration IDs**: `202602150030` (agents), `202602150031` (skills), `202602150032` (tasks), `202602150033` (workflows), `202602150034` (users)

**Each migration adds**:
- New column(s) via `tx.Migrator().AddColumn()`
- FK constraint: `project_id` → `projects(id)` ON DELETE SET NULL
- Index on `project_id`

**Test criteria**:
- [ ] POST agent/skill/task/workflow with `project_id` round-trips correctly
- [ ] PATCH can update `project_id`
- [ ] POST workflow with `branch` and `path` round-trips
- [ ] POST user with `groups` (string array) round-trips
- [ ] Existing tests pass (backward compatible — new fields are nullable)
- [ ] `make test` passes

---

### WP4 — Session Schema Expansion

**Scope**: Add ~24 new fields to Session (the largest single change). Split into data fields (API-mutable) and status fields (readOnly, CP-owned).

**Files modified**:
- `plugins/sessions/model.go` — add all fields + `SessionStatusPatchRequest` struct
- `plugins/sessions/migration.go` — new migration adding columns
- `plugins/sessions/handler.go` — extend Patch to handle new data fields
- `plugins/sessions/presenter.go` — extend ConvertSession/PresentSession
- `plugins/sessions/factory_test.go` — update factory with new fields
- `plugins/sessions/integration_test.go` — add tests for new fields
- `openapi/openapi.sessions.yaml` — add all fields, mark status fields `readOnly: true`

**Data fields added to model**:
```go
Repos                *datatypes.JSON `json:"repos"`
Interactive          *bool           `json:"interactive"`
Timeout              *int            `json:"timeout"`
LlmModel             *string         `json:"llm_model"`
LlmTemperature       *float64        `json:"llm_temperature"`
LlmMaxTokens         *int            `json:"llm_max_tokens"`
ParentSessionId      *string         `json:"parent_session_id"`
BotAccountName       *string         `json:"bot_account_name"`
ResourceOverrides    *datatypes.JSON `json:"resource_overrides"`
EnvironmentVariables *datatypes.JSON `json:"environment_variables"`
SessionLabels        *datatypes.JSON `json:"labels" gorm:"column:labels"`
SessionAnnotations   *datatypes.JSON `json:"annotations" gorm:"column:annotations"`
ProjectId            *string         `json:"project_id"`
```

**Status fields added to model** (readOnly — excluded from `SessionPatchRequest`):
```go
Phase               *string         `json:"phase"`
StartTime           *time.Time      `json:"start_time"`
CompletionTime      *time.Time      `json:"completion_time"`
SdkSessionId        *string         `json:"sdk_session_id"`
SdkRestartCount     *int            `json:"sdk_restart_count"`
Conditions          *datatypes.JSON `json:"conditions"`
ReconciledRepos     *datatypes.JSON `json:"reconciled_repos"`
ReconciledWorkflow  *datatypes.JSON `json:"reconciled_workflow"`
KubeCrName          *string         `json:"kube_cr_name"`
KubeCrUid           *string         `json:"kube_cr_uid"`
KubeNamespace       *string         `json:"kube_namespace"`
```

**BeforeCreate hook update**: Set `kube_cr_name = session.id` (per CR Naming contract).

**Migration ID**: `202602150040`

**Migration adds**:
- All new columns
- FK: `project_id` → `projects(id)` ON DELETE SET NULL
- FK: `parent_session_id` → `sessions(id)` ON DELETE SET NULL
- Indexes on `project_id`, `phase`, `parent_session_id`, `kube_cr_name`

**Test criteria**:
- [ ] POST with new data fields (repos jsonb, interactive bool, etc.) round-trips
- [ ] Status fields returned in GET response (initially null)
- [ ] Status fields NOT accepted in regular PATCH (ignored or rejected)
- [ ] `kube_cr_name` auto-set to `session.id` in BeforeCreate
- [ ] `parent_session_id` self-referencing FK works
- [ ] Existing session tests still pass (all new fields nullable = backward compatible)
- [ ] `make test` passes

---

## Phase 1c: Session Lifecycle

### WP5 — Session Status Write-back Endpoint

**Scope**: `PATCH /sessions/{id}/status` — CP-only endpoint for syncing runtime fields from K8s CR status back to Postgres. Per contract: service account auth required.

**Files modified**:
- `plugins/sessions/model.go` — add `SessionStatusPatchRequest` struct
- `plugins/sessions/handler.go` — add `PatchStatus` method
- `plugins/sessions/service.go` — add `UpdateStatus` method
- `plugins/sessions/plugin.go` — register `/sessions/{id}/status` route

**Route**: `PATCH /api/ambient-api-server/v1/sessions/{id}/status`

**`SessionStatusPatchRequest`**:
```go
type SessionStatusPatchRequest struct {
    Phase              *string         `json:"phase,omitempty"`
    StartTime          *time.Time      `json:"start_time,omitempty"`
    CompletionTime     *time.Time      `json:"completion_time,omitempty"`
    SdkSessionId       *string         `json:"sdk_session_id,omitempty"`
    SdkRestartCount    *int            `json:"sdk_restart_count,omitempty"`
    Conditions         *datatypes.JSON `json:"conditions,omitempty"`
    ReconciledRepos    *datatypes.JSON `json:"reconciled_repos,omitempty"`
    ReconciledWorkflow *datatypes.JSON `json:"reconciled_workflow,omitempty"`
    KubeCrUid          *string         `json:"kube_cr_uid,omitempty"`
    KubeNamespace      *string         `json:"kube_namespace,omitempty"`
}
```

**Test criteria**:
- [ ] PATCH `/sessions/{id}/status` with `{phase: "Running"}` updates `phase`
- [ ] GET after status PATCH shows updated status fields
- [ ] Regular PATCH `/sessions/{id}` does NOT update status fields
- [ ] Status PATCH preserves data fields (no clobbering)
- [ ] Status PATCH on non-existent session returns 404
- [ ] `make test` passes

---

### WP6 — Session Start/Stop Actions

**Scope**: `POST /sessions/{id}/start` and `POST /sessions/{id}/stop`. These set `phase` in Postgres. pg_notify deferred to CP integration (Phase 1b coordination).

**Files modified**:
- `plugins/sessions/handler.go` — add `Start`, `Stop` methods
- `plugins/sessions/service.go` — add `Start`, `Stop` methods with phase validation
- `plugins/sessions/plugin.go` — register action routes

**Routes**:
- `POST /api/ambient-api-server/v1/sessions/{id}/start`
- `POST /api/ambient-api-server/v1/sessions/{id}/stop`

**Logic**:
- `Start`: validate current phase is `nil`/empty or `Stopped` → set `phase=Pending`, emit update event
- `Stop`: validate current phase is `Running` or `Creating` → set `phase=Stopping`, emit update event
- Invalid transitions return 409 Conflict with reason

**Test criteria**:
- [ ] POST `/sessions/{id}/start` on new session sets `phase=Pending`, returns 200
- [ ] POST `/sessions/{id}/stop` on running session sets `phase=Stopping`, returns 200
- [ ] Start on already-running session returns 409
- [ ] Stop on already-stopped session returns 409
- [ ] GET after start/stop reflects updated phase
- [ ] `make test` passes

---

## Phase 1d: OpenAPI + Client Regeneration

### WP7 — OpenAPI Spec Update + Client Regen

**Scope**: Update all OpenAPI YAML specs with new/expanded schemas. Regenerate Go client. This runs incrementally after each WP but is listed last because it depends on all schema changes being defined.

**Files modified**:
- `openapi/openapi.yaml` — add Project, ProjectSettings paths + schema refs
- `openapi/openapi.projects.yaml` — new file
- `openapi/openapi.projectSettings.yaml` — new file
- `openapi/openapi.sessions.yaml` — expand Session schema (data + readOnly status fields), add `SessionStatusPatchRequest`, add `/sessions/{id}/status` path, add `/sessions/{id}/start` + `/sessions/{id}/stop` paths
- `openapi/openapi.agents.yaml` — add `project_id`
- `openapi/openapi.skills.yaml` — add `project_id`
- `openapi/openapi.tasks.yaml` — add `project_id`
- `openapi/openapi.workflows.yaml` — add `project_id`, `branch`, `path`
- `openapi/openapi.users.yaml` — add `groups`
- `pkg/api/openapi/` — regenerated (do not hand-edit)

**Test criteria**:
- [ ] `make generate` succeeds
- [ ] `go build ./...` succeeds with regenerated client
- [ ] All integration tests pass
- [ ] OpenAPI spec validates (no broken $refs, no missing schemas)
- [ ] Status fields on Session marked `readOnly: true`
- [ ] `make test` passes

---

## Overlord Concerns — API Session Responses

Responses to the 8 blocking concerns from the senior review. These shaped the plan above.

### Concern 1 — Transition Strategy

**Accepted.** A one-page transition plan is needed before writing production code. However, Phase 1 is the data layer build — it produces a working API server with tests but does NOT replace the backend. The transition plan (strangler-fig, dual-write, data migration, traffic cutover, rollback) is a Phase 2+ concern that should be documented before any frontend/SDK switchover. **Action**: API + CP will produce a TRANSITION_PLAN.md before Phase 1c merges to main. Phase 1a/1b can proceed because they only add capabilities — they don't replace anything.

### Concern 2 — Phase 1 Scope

**Accepted and implemented.** Phase 1 is now split into 4 sub-phases (1a/1b/1c/1d), each independently deployable and testable. See dependency graph above. WP0 (migration hardening) comes first per Concern 4.

### Concern 3 — CP Has Zero Tests

**Acknowledged.** This is a CP session concern. API session's response: the contracts and endpoints defined here are testable from the API side. CP must add tests before implementing the write path. The status write-back endpoint (WP5) will include API-side integration tests that CP can reference.

### Concern 4 — Migrations Have No FK/Indexes

**Accepted and implemented.** WP0 (Migration Hardening) is now the first work package. Investigated: confirmed that all 8 existing migrations use bare `AutoMigrate` with zero FK constraints, zero indexes, and zero unique constraints. The SQL schema file (`ambient-data-model.sql`) defines 8 FKs, 4 unique constraints, and 16 indexes that the Go migrations don't enforce. WP0 adds a catch-up migration.

### Concern 5 — AG-UI Spike

**Accepted in principle.** AG-UI is genuinely the hardest part. A spike to validate that the rh-trex-ai framework supports SSE streaming + background goroutines should happen before Phase 2 begins. However, it does NOT block Phase 1 (data layer). **Action**: Schedule an AG-UI spike as a parallel track during Phase 1c.

### Concern 6 — Definition of Done

**Accepted.** A "backend replacement complete" checklist should be added to Contracts. Proposed criteria:
1. All 80+ backend endpoints have equivalents in API server
2. Frontend updated to call API server
3. SDK talks directly to API server (public-api removed)
4. Existing sessions migrated (etcd → Postgres)
5. OAuth callback URLs re-registered
6. Operator unchanged, still works with AgenticSession CRs
7. All integration tests pass
8. 72-hour soak test in staging with no regressions

**Action**: Add to Contracts section after all sessions acknowledge.

### Concern 7 — rh-trex-ai Local Replace

**Context**: `go.mod` uses `replace github.com/openshift-online/rh-trex-ai => /home/mturansk/projects/src/github.com/openshift-online/rh-trex-ai`. Local clone is on `main` at commit `04bc1dd`. The pinned pseudo-version (`v0.0.0-20260211222339-04bc1dddc1f7`) matches the local HEAD.

**Plan**: For PoC, the local replace stays — it enables fast iteration on framework features. Before Phase 2, we will either: (a) publish tagged releases of rh-trex-ai and pin to them, or (b) vendor the dependency. CI will be configured to clone the framework at the pinned commit.

**Action**: Document the commit contract in Contracts section. CI pipeline must `git clone` rh-trex-ai at the pinned commit before building.

### Concern 8 — Polling Scalability

**Accepted.** CP must paginate the informer before Phase 1b (reconcile loop). API's contribution: add `pg_notify` channel contract to Contracts.

**pg_notify contract** (proposed):
- Channel: `ambient_session_events`
- Payload: JSON `{"id": "<session_id>", "event": "created|updated|deleted"}`
- Triggered by: Session Create, Replace, Delete in the service layer (after DB write)
- Implementation: `tx.Exec("SELECT pg_notify('ambient_session_events', $1)", payload)` in `SessionService`
- CP listens via `LISTEN ambient_session_events` on a dedicated connection

**Action**: Add to Contracts after CP acknowledges.

---

## Phases 2–4 (Stubs)

These are fully specified in [DATA_MODEL_COMPARISON.md, Section 8](DATA_MODEL_COMPARISON.md#8-endpoint-migration-roadmap). Implementation plans will be written when Phase 1 is complete.

| Phase | Scope | Prerequisites |
|-------|-------|--------------|
| **Phase 2** | Operational endpoints (permissions, secrets, project keys, auth integrations) | Phase 1 complete, transition plan approved |
| **Phase 3** | Runtime proxies (AG-UI, workspace, git ops, content service) | Phase 2 complete, AG-UI spike validated |
| **Phase 4** | Frontend/SDK switchover, public-api removal | Phases 1–3 complete, Definition of Done met |
