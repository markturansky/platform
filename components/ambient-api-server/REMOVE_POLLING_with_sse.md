# Plan: Replace SDK Polling with Server-Sent Events

## Problem

Both Go and Python SDKs use HTTP polling in `WaitForCompletion` / `wait_for_completion` to monitor session state. This wastes bandwidth, adds latency (up to `pollInterval` seconds per state change), and scales poorly with concurrent sessions.

## Solution

Add a Server-Sent Events (SSE) endpoint to `ambient-api-server`. SDK clients connect once and receive real-time push notifications for session state transitions. HTTP polling becomes the fallback, not the primary mechanism.

SSE over WebSocket because:
- Data flow is unidirectional (server → client)
- Works through proxies and load balancers without special config
- No new dependencies — `net/http` + `http.Flusher` is sufficient
- Already proven in the platform (`components/backend/websocket/agui.go`)

---

## Prerequisites

### P1. Add `status` field to Session model

The Session model (`plugins/sessions/model.go:8`) has no lifecycle state. The SDK expects `pending`, `running`, `completed`, `failed` but the API server doesn't track this.

**Add to `Session` struct:**
```go
Status string `json:"status" gorm:"default:'pending'"`
```

**Add to `SessionPatchRequest`:**
```go
Status *string `json:"status,omitempty"`
```

**Add a gormigrate migration** in `plugins/sessions/migration.go` to add the column.

**Update the presenter** (`plugins/sessions/presenter.go`) to map `Status` between model and OpenAPI types.

**Update OpenAPI spec** (`openapi/`) to include `status` on the Session schema with enum values.

---

## Implementation Steps

### Step 1: SSE Event Hub (new file: `pkg/sse/hub.go`)

An in-memory fan-out hub that bridges PostgreSQL LISTEN/NOTIFY to SSE client connections.

```
pg_notify("events") → ControllersServer → OnUpsert/OnDelete → Hub.Broadcast()
                                                                    ↓
                                                          subscriber channels
                                                                    ↓
                                                          SSE handler → HTTP response
```

**Design (modeled on `backend/websocket/agui.go:35-76`):**

```go
package sse

type Event struct {
    Type     string      // "session.updated", "session.deleted"
    ID       string      // resource ID
    Data     interface{} // serialized resource or patch
}

type Hub struct {
    mu          sync.RWMutex
    subscribers map[string]map[chan Event]struct{} // key = resource ID
}

func (h *Hub) Subscribe(resourceID string) chan Event   // create buffered chan, add to map
func (h *Hub) Unsubscribe(resourceID string, ch chan Event) // remove from map, close chan
func (h *Hub) Broadcast(resourceID string, event Event) // fan-out to all subscribers for this ID
```

- Buffered channels (`make(chan Event, 32)`) with non-blocking send (drop if full, log warning)
- `sync.RWMutex` for concurrent read/write safety
- Subscribers keyed by resource ID for targeted delivery

### Step 2: Wire Hub into Session Controller

**File: `plugins/sessions/service.go`**

Inject the Hub into `sqlSessionService`. Replace the stub `OnUpsert` (line 49) and `OnDelete` (line 62) with:

```go
func (s *sqlSessionService) OnUpsert(ctx context.Context, id string) error {
    session, err := s.sessionDao.Get(ctx, id)
    if err != nil {
        return err
    }
    s.hub.Broadcast(id, sse.Event{
        Type: "session.updated",
        ID:   id,
        Data: PresentSession(session), // reuse existing presenter
    })
    return nil
}

func (s *sqlSessionService) OnDelete(ctx context.Context, id string) error {
    s.hub.Broadcast(id, sse.Event{
        Type: "session.deleted",
        ID:   id,
    })
    return nil
}
```

The event flow is already wired: `service.Create()` → `EventService.Create()` → `pg_notify` → `ControllersServer` listener → `OnUpsert`. No changes needed to the event pipeline.

### Step 3: SSE HTTP Handler (new file: `pkg/sse/handler.go`)

**Endpoint:** `GET /api/ambient-api-server/v1/sessions/{id}/events`

```go
func (h *SSEHandler) StreamSessionEvents(w http.ResponseWriter, r *http.Request) {
    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    sessionID := mux.Vars(r)["id"]

    // Validate session exists
    _, err := h.sessionService.Get(r.Context(), sessionID)
    if err != nil {
        // 404
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")

    // Send current state as first event (snapshot)
    session, _ := h.sessionService.Get(r.Context(), sessionID)
    writeSSEEvent(w, flusher, "session.snapshot", session)

    // Subscribe to future changes
    ch := h.hub.Subscribe(sessionID)
    defer h.hub.Unsubscribe(sessionID, ch)

    keepalive := time.NewTicker(15 * time.Second)
    defer keepalive.Stop()

    for {
        select {
        case <-r.Context().Done():
            return
        case event, ok := <-ch:
            if !ok {
                return
            }
            writeSSEEvent(w, flusher, event.Type, event.Data)
            // Close stream on terminal states
            if isTerminal(event) {
                return
            }
        case <-keepalive.C:
            fmt.Fprintf(w, ": keepalive\n\n")
            flusher.Flush()
        }
    }
}

func writeSSEEvent(w http.ResponseWriter, f http.Flusher, eventType string, data interface{}) {
    jsonData, _ := json.Marshal(data)
    fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, jsonData)
    f.Flush()
}
```

### Step 4: Route Registration (bypass problematic middleware)

**Problem:** The `apiV1Router` applies `db.TransactionMiddleware` and `gorillahandlers.CompressHandler` (see `rh-trex-ai/pkg/server/routebuilder.go:61-67`). Both break SSE:
- `TransactionMiddleware` holds a DB transaction open for the request lifetime — SSE connections are long-lived
- `CompressHandler` buffers output, preventing streaming

**Solution:** Register the SSE route on a dedicated subrouter that skips these middleware. Modify `plugins/sessions/plugin.go` route registration:

```go
// Standard CRUD routes (existing, on apiV1Router with all middleware)
sessionsRouter := apiV1Router.PathPrefix("/sessions").Subrouter()
sessionsRouter.HandleFunc("", sessionHandler.List).Methods(http.MethodGet)
// ... existing routes ...

// SSE route — registered WITHOUT TransactionMiddleware and CompressHandler
// Option A: Register on apiRouter (parent of apiV1Router, no DB/compress middleware)
// Option B: Add a new RegisterStreamRoutes() function to the upstream framework
//           that provides a middleware-free subrouter
sessionsRouter.HandleFunc("/{id}/events", sseHandler.StreamSessionEvents).Methods(http.MethodGet)
```

**Note:** Since `sessionsRouter` is a subrouter of `apiV1Router`, it inherits the middleware. The cleanest solution is to extend the upstream `RegisterRoutes` signature to also pass the parent `apiRouter` (or a streaming-specific router), or to add a `RegisterStreamRoutes()` registration function in the upstream framework that provides a clean router. This is a small change to `rh-trex-ai/pkg/server/routes.go` and `routebuilder.go`.

**Upstream change to `rh-trex-ai/pkg/server/routes.go`:**
```go
type StreamRouteRegistrationFunc func(apiV1Router *mux.Router, services ServicesInterface)

var streamRouteRegistry = make(map[string]StreamRouteRegistrationFunc)

func RegisterStreamRoutes(name string, registrationFunc StreamRouteRegistrationFunc) {
    streamRouteRegistry[name] = registrationFunc
}
```

**Upstream change to `rh-trex-ai/pkg/server/routebuilder.go`:**
```go
// After line 67 (CompressHandler), before LoadDiscoveredRoutes:
// Create a streaming router without TransactionMiddleware and CompressHandler
apiV1StreamRouter := apiRouter.PathPrefix("/v1").Subrouter()
apiV1StreamRouter.Use(MetricsMiddleware)
LoadDiscoveredStreamRoutes(apiV1StreamRouter, services)
```

### Step 5: Update Go SDK

**File: `go-sdk/client/client.go`**

Add `WatchSession()` that uses SSE, and update `WaitForCompletion()` to use it with HTTP polling fallback:

```go
func (c *Client) WatchSession(ctx context.Context, sessionID string) (<-chan *types.SessionEvent, error) {
    url := fmt.Sprintf("%s/v1/sessions/%s/events", c.baseURL, sessionID)
    req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    req.Header.Set("Authorization", "Bearer "+c.token.String())
    req.Header.Set("X-Ambient-Project", c.project)
    req.Header.Set("Accept", "text/event-stream")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }

    events := make(chan *types.SessionEvent, 16)
    go func() {
        defer close(events)
        defer resp.Body.Close()
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            line := scanner.Text()
            // Parse SSE format: "event: type\ndata: json\n\n"
            // Emit typed events on channel
        }
    }()
    return events, nil
}

func (c *Client) WaitForCompletion(ctx context.Context, sessionID string, pollInterval time.Duration) (*types.SessionResponse, error) {
    // Try SSE first
    events, err := c.WatchSession(ctx, sessionID)
    if err == nil {
        for event := range events {
            if event.Session.Status == types.StatusCompleted || event.Session.Status == types.StatusFailed {
                return &event.Session, nil
            }
        }
    }
    // Fallback to polling (existing implementation)
    return c.waitForCompletionPolling(ctx, sessionID, pollInterval)
}
```

### Step 6: Update Python SDK

**File: `python-sdk/ambient_platform/client.py`**

Add `watch_session()` generator and update `wait_for_completion()`:

```python
def watch_session(self, session_id: str) -> Iterator[SessionEvent]:
    """Stream session events via SSE."""
    url = f"{self.base_url}/v1/sessions/{session_id}/events"
    headers = {
        "Authorization": f"Bearer {self._token}",
        "X-Ambient-Project": self._project,
        "Accept": "text/event-stream",
    }
    with httpx.stream("GET", url, headers=headers, timeout=None) as response:
        response.raise_for_status()
        for line in response.iter_lines():
            # Parse SSE format
            if line.startswith("data: "):
                data = json.loads(line[6:])
                yield SessionEvent.from_dict(data)

def wait_for_completion(self, session_id: str, poll_interval: float = 5.0, timeout: float = None) -> SessionResponse:
    """Wait for session completion. Uses SSE with polling fallback."""
    try:
        for event in self.watch_session(session_id):
            if event.session.status in (StatusCompleted, StatusFailed):
                return event.session
    except Exception:
        pass
    # Fallback to polling (existing implementation)
    return self._wait_for_completion_polling(session_id, poll_interval, timeout)
```

---

## SSE Wire Format

```
event: session.snapshot
data: {"id":"abc123","status":"pending","name":"my-session",...}

event: session.updated
data: {"id":"abc123","status":"running","name":"my-session",...}

event: session.updated
data: {"id":"abc123","status":"completed","name":"my-session","result":"..."}

event: session.deleted
data: {"id":"abc123"}

: keepalive

```

- `event:` field enables `EventSource.addEventListener()` in browser clients
- `data:` is always JSON, always the full resource representation (not a diff)
- `: keepalive` is an SSE comment, ignored by clients, keeps connection alive through proxies
- Terminal events (`status=completed|failed`, or `session.deleted`) are sent, then the server closes the stream

---

## Middleware Considerations

| Middleware | Problem for SSE | Mitigation |
|---|---|---|
| `db.TransactionMiddleware` | Holds a DB tx open for the entire request lifetime | SSE routes use a separate router that skips this middleware; SSE handler opens its own short-lived DB calls as needed |
| `gorillahandlers.CompressHandler` | Buffers response body, prevents streaming | SSE router skips CompressHandler |
| `auth.JWTMiddleware` | No problem — authenticates once at connection time | Apply to SSE router normally |
| CORS (`gorillahandlers.CORS`) | No problem — applied at the top-level `mainHandler` | Works as-is; may need to add `text/event-stream` to allowed content types |

---

## Task Sequence

| # | Task | Files Modified | Depends On |
|---|------|---------------|------------|
| 1 | Add `Status` field to Session model + migration | `plugins/sessions/model.go`, `migration.go`, `presenter.go` | — |
| 2 | Update OpenAPI spec with `status` enum | `openapi/*.yaml` | 1 |
| 3 | Regenerate OpenAPI client | `pkg/api/openapi/` (generated) | 2 |
| 4 | Create SSE Hub (`pkg/sse/hub.go`) | new file | — |
| 5 | Create SSE handler (`pkg/sse/handler.go`) | new file | 4 |
| 6 | Add `RegisterStreamRoutes` to upstream framework | `rh-trex-ai/pkg/server/routes.go`, `routebuilder.go` | — |
| 7 | Wire Hub into Session service + controller | `plugins/sessions/service.go`, `plugin.go` | 4, 5, 6 |
| 8 | Register SSE route via `RegisterStreamRoutes` | `plugins/sessions/plugin.go` | 6, 7 |
| 9 | Add `WatchSession()` to Go SDK + update `WaitForCompletion()` | `go-sdk/client/client.go`, `types/types.go` | 5 |
| 10 | Add `watch_session()` to Python SDK + update `wait_for_completion()` | `python-sdk/ambient_platform/client.py`, `types.py` | 5 |
| 11 | Integration tests: SSE endpoint | `plugins/sessions/*_test.go` or `test/integration/` | 7, 8 |
| 12 | SDK tests: SSE client with mock server | `go-sdk/`, `python-sdk/` | 9, 10 |
| 13 | Update SDK specification doc | `AMBIENT_SDK_SPECIFICATION.md` | 9, 10 |

---

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Upstream framework changes rejected | Blocks stream route registration | Alternative: monkey-patch the router in the session plugin's `init()` by accessing the parent router directly, or register SSE on a completely separate HTTP server (like metrics/healthcheck) |
| Connection leak if clients don't close | Memory growth from orphaned subscriber channels | Hub garbage-collects subscribers with no activity after 30 minutes; `defer Unsubscribe()` in handler; monitor subscriber count via metrics |
| Proxy buffering kills SSE | Events don't reach client | `X-Accel-Buffering: no` header (nginx), `Cache-Control: no-cache`, document proxy config requirements |
| Hub is in-memory, single-process | Horizontal scaling breaks fan-out | Phase 2: Use PostgreSQL LISTEN/NOTIFY directly per API server instance (each instance has its own listener, gets all events). The Hub is per-process and that's correct — each server fans out to its own connected clients |
| Session model has no status today | SDK can't observe meaningful transitions | Prerequisite P1 must land first |
