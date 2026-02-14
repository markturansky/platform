# Backend & Operator Development Context

**When to load:** Working on Go backend API, handlers, Kubernetes operator, or reconciliation logic

## Quick Reference

- **Language:** Go 1.21+
- **Backend Framework:** Gin (HTTP router)
- **K8s Client:** client-go + dynamic client
- **Backend Files:** `components/backend/handlers/*.go`, `components/backend/types/*.go`
- **Operator Files:** `components/operator/internal/handlers/*.go`, `components/operator/internal/config/*.go`

## Critical Rules (Never Violate)

### 1. User Token Authentication Required

```go
reqK8s, reqDyn := GetK8sClientsForRequest(c)
if reqK8s == nil {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or missing token"})
    c.Abort()
    return
}
```

**FORBIDDEN:** Using backend service account (`DynamicClient`, `K8sClient`) for user-initiated operations

**Backend service account ONLY for:**

- Writing CRs after validation (handlers/sessions.go:417)
- Minting tokens/secrets for runners (handlers/sessions.go:449)
- Cross-namespace operations backend is authorized for

### 2. Never Panic in Production Code

- FORBIDDEN: `panic()` in handlers, reconcilers, or any production path
- REQUIRED: `return fmt.Errorf("failed to X: %w", err)`
- REQUIRED: `log.Printf("Operation failed: %v", err)` before returning

### 3. Token Security and Redaction

```go
// NEVER log the token itself
log.Printf("Processing request with token (len=%d)", len(token))
// Redact in URL paths
path = strings.Split(path, "?")[0] + "?token=[REDACTED]"
```

**Token Redaction Pattern:** See `server/server.go:22-34`

### 4. Type-Safe Unstructured Access

```go
// REQUIRED: Use unstructured helpers with three-value returns
spec, found, err := unstructured.NestedMap(obj.Object, "spec")
if !found || err != nil {
    return fmt.Errorf("spec not found")
}
```

**FORBIDDEN:** Direct type assertions: `obj.Object["spec"].(map[string]interface{})`

### 5. OwnerReferences for Resource Lifecycle

```go
ownerRef := v1.OwnerReference{
    APIVersion: obj.GetAPIVersion(),
    Kind:       obj.GetKind(),
    Name:       obj.GetName(),
    UID:        obj.GetUID(),
    Controller: boolPtr(true),
    // BlockOwnerDeletion: intentionally omitted (permission issues)
}
```

REQUIRED on all child resources (Jobs, Secrets, PVCs, Services).

## Exception: Public API Gateway Service

The `components/public-api/` service does NOT follow the backend patterns above. This is intentional:

- **No K8s Clients**: Does NOT use `GetK8sClientsForRequest()` or access Kubernetes directly
- **No RBAC Permissions**: ServiceAccount has NO RoleBindings
- **Token Forwarding Only**: Proxies requests to backend with user's token in `Authorization` header
- **Backend Validates**: All K8s operations and RBAC enforcement happen in the backend service

The public-api is a thin shim: extract token, extract project context, validate input, forward with auth headers.

## Package Organization

**Backend Structure** (`components/backend/`):

```
backend/
├── handlers/          # HTTP handlers grouped by resource
│   ├── sessions.go    # AgenticSession CRUD + lifecycle
│   ├── projects.go    # Project management
│   ├── rfe.go         # RFE workflows
│   ├── helpers.go     # Shared utilities (StringPtr, etc.)
│   └── middleware.go  # Auth, validation, RBAC
├── types/             # Type definitions (no business logic)
├── server/            # Server setup, CORS, middleware
├── k8s/               # K8s resource templates
├── git/, github/      # External integrations
├── websocket/         # Real-time messaging
├── routes.go          # HTTP route registration
└── main.go            # Wiring, dependency injection
```

**Operator Structure** (`components/operator/`):

```
operator/
├── internal/
│   ├── config/        # K8s client init, config loading
│   ├── types/         # GVR definitions, resource helpers
│   ├── handlers/      # Watch handlers (sessions, namespaces, projectsettings)
│   └── services/      # Reusable services (PVC provisioning, etc.)
└── main.go            # Watch coordination
```

**Rules:**

- Handlers contain HTTP/watch logic ONLY
- Types are pure data structures
- Business logic in separate service packages
- No cyclic dependencies between packages

## API Design Patterns

**Project-Scoped Endpoints:**

```go
r.GET("/api/projects/:projectName/agentic-sessions", ValidateProjectContext(), ListSessions)
r.POST("/api/projects/:projectName/agentic-sessions", ValidateProjectContext(), CreateSession)
r.GET("/api/projects/:projectName/agentic-sessions/:sessionName", ValidateProjectContext(), GetSession)
```

**Middleware Chain** (order matters):

```go
Recovery → Logging → CORS → Identity → Validation → Handler
```

**Response Patterns:**

```go
c.JSON(http.StatusOK, gin.H{"items": sessions})
c.JSON(http.StatusCreated, gin.H{"message": "Session created", "name": name, "uid": uid})
c.Status(http.StatusNoContent)
c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
```

## Operator Patterns

**Watch Loop with Reconnection:**

```go
for {
    watcher, err := config.DynamicClient.Resource(gvr).Watch(ctx, v1.ListOptions{})
    if err != nil {
        log.Printf("Failed to create watcher: %v", err)
        time.Sleep(5 * time.Second)
        continue
    }
    for event := range watcher.ResultChan() {
        switch event.Type {
        case watch.Added, watch.Modified:
            obj := event.Object.(*unstructured.Unstructured)
            handleEvent(obj)
        case watch.Deleted:
            // Handle cleanup
        }
    }
    watcher.Stop()
    time.Sleep(2 * time.Second)
}
```

**Reconciliation Pattern:**

1. Verify resource still exists (race condition check)
2. Get current phase/status
3. Only reconcile if in expected state (avoid duplicates)
4. Create resources idempotently (check existence first)
5. Update status via `UpdateStatus` subresource

**Status Updates** (use UpdateStatus subresource):

```go
_, err = config.DynamicClient.Resource(gvr).Namespace(namespace).UpdateStatus(ctx, obj, v1.UpdateOptions{})
if errors.IsNotFound(err) {
    return nil  // Resource deleted during update
}
```

**Goroutine Monitoring:**

- Always check if parent resource still exists (exit if deleted)
- Sleep between checks (5 seconds typical)
- Clean up after completion

## Common Mistakes to Avoid

**Backend:**

- Using service account client for user operations (always use user token)
- Not checking if user-scoped client creation succeeded
- Logging full token values (use `len(token)` instead)
- Not validating project access in middleware
- Type assertions without checking: `val := obj["key"].(string)` (use `val, ok := ...`)
- Not setting OwnerReferences (causes resource leaks)
- Treating IsNotFound as fatal error during cleanup
- Exposing internal error details to API responses (use generic messages)

**Operator:**

- Not reconnecting watch on channel close
- Processing events without verifying resource still exists
- Updating status on main object instead of /status subresource
- Not checking current phase before reconciliation (causes duplicate resources)
- Creating resources without idempotency checks
- Goroutine leaks (not exiting monitor when resource deleted)
- Using `panic()` in watch/reconciliation loops
- Not setting SecurityContext on Job pods

## Common Tasks

### Adding a New API Endpoint

1. **Define route:** `routes.go` with middleware chain
2. **Create handler:** `handlers/[resource].go`
3. **Validate project context:** Use `ValidateProjectContext()` middleware
4. **Get user clients:** `GetK8sClientsForRequest(c)`
5. **Perform operation:** Use `reqDyn` for K8s resources
6. **Return response:** Structured JSON with appropriate status code

### Adding a New Custom Resource Field

1. **Update CRD:** `components/manifests/base/[resource]-crd.yaml`
2. **Update types:** `components/backend/types/[resource].go`
3. **Update handlers:** Extract/validate new field in handlers
4. **Update operator:** Handle new field in reconciliation
5. **Test:** Create sample CR with new field

## Pre-Commit Checklist

- [ ] All user operations use `GetK8sClientsForRequest`
- [ ] RBAC checks performed before resource access
- [ ] No tokens in logs
- [ ] Errors logged with context, appropriate HTTP status codes
- [ ] Type-safe unstructured access (`unstructured.Nested*` helpers)
- [ ] OwnerReferences set on all child resources
- [ ] Status updates use `UpdateStatus` subresource, handle IsNotFound
- [ ] `gofmt -w .` applied
- [ ] `go vet ./...` passes
- [ ] `golangci-lint run` passes

## Key Files

**Backend:**

- `handlers/sessions.go` - Complete session lifecycle, user/SA client usage
- `handlers/middleware.go` - Auth patterns, token extraction, RBAC
- `handlers/helpers.go` - Utility functions (StringPtr, BoolPtr)
- `types/common.go` - Type definitions
- `server/server.go` - Server setup, middleware chain, token redaction
- `routes.go` - HTTP route definitions and registration

**Operator:**

- `internal/handlers/sessions.go` - Watch loop, reconciliation, status updates
- `internal/config/config.go` - K8s client initialization
- `internal/types/resources.go` - GVR definitions
- `internal/services/infrastructure.go` - Reusable services
