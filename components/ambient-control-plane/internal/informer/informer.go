package informer

import (
	"context"
	"sync"
	"time"

	openapi "github.com/ambient/platform/components/ambient-api-server/pkg/api/openapi"
	"github.com/rs/zerolog"
)

type EventType string

const (
	EventAdded    EventType = "ADDED"
	EventModified EventType = "MODIFIED"
	EventDeleted  EventType = "DELETED"
)

type ResourceEvent struct {
	Type      EventType
	Resource  string
	Object    interface{}
	OldObject interface{}
}

type EventHandler func(ctx context.Context, event ResourceEvent) error

type Informer struct {
	client        *openapi.APIClient
	pollInterval  time.Duration
	handlers      map[string][]EventHandler
	mu            sync.RWMutex
	logger        zerolog.Logger
	sessionCache  map[string]openapi.Session
	workflowCache map[string]openapi.Workflow
	taskCache     map[string]openapi.Task
}

func New(client *openapi.APIClient, pollInterval time.Duration, logger zerolog.Logger) *Informer {
	return &Informer{
		client:        client,
		pollInterval:  pollInterval,
		handlers:      make(map[string][]EventHandler),
		logger:        logger.With().Str("component", "informer").Logger(),
		sessionCache:  make(map[string]openapi.Session),
		workflowCache: make(map[string]openapi.Workflow),
		taskCache:     make(map[string]openapi.Task),
	}
}

func (inf *Informer) RegisterHandler(resource string, handler EventHandler) {
	inf.mu.Lock()
	defer inf.mu.Unlock()
	inf.handlers[resource] = append(inf.handlers[resource], handler)
}

func (inf *Informer) Run(ctx context.Context) error {
	inf.logger.Info().
		Dur("poll_interval", inf.pollInterval).
		Msg("starting informer loop")

	if err := inf.syncAll(ctx); err != nil {
		inf.logger.Warn().Err(err).Msg("initial sync failed, will retry")
	}

	ticker := time.NewTicker(inf.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			inf.logger.Info().Msg("informer shutting down")
			return ctx.Err()
		case <-ticker.C:
			if err := inf.syncAll(ctx); err != nil {
				inf.logger.Error().Err(err).Msg("sync cycle failed")
			}
		}
	}
}

func (inf *Informer) syncAll(ctx context.Context) error {
	if err := inf.syncSessions(ctx); err != nil {
		return err
	}
	if err := inf.syncWorkflows(ctx); err != nil {
		return err
	}
	if err := inf.syncTasks(ctx); err != nil {
		return err
	}
	return nil
}

func (inf *Informer) syncSessions(ctx context.Context) error {
	list, _, err := inf.client.DefaultAPI.ApiAmbientApiServerV1SessionsGet(ctx).Execute()
	if err != nil {
		return err
	}

	currentIDs := make(map[string]bool)
	for _, session := range list.Items {
		id := session.GetId()
		currentIDs[id] = true

		if existing, found := inf.sessionCache[id]; found {
			if session.GetUpdatedAt() != existing.GetUpdatedAt() {
				inf.sessionCache[id] = session
				inf.dispatch(ctx, ResourceEvent{
					Type:      EventModified,
					Resource:  "sessions",
					Object:    session,
					OldObject: existing,
				})
			}
		} else {
			inf.sessionCache[id] = session
			inf.dispatch(ctx, ResourceEvent{
				Type:     EventAdded,
				Resource: "sessions",
				Object:   session,
			})
		}
	}

	for id, session := range inf.sessionCache {
		if !currentIDs[id] {
			delete(inf.sessionCache, id)
			inf.dispatch(ctx, ResourceEvent{
				Type:     EventDeleted,
				Resource: "sessions",
				Object:   session,
			})
		}
	}

	return nil
}

func (inf *Informer) syncWorkflows(ctx context.Context) error {
	list, _, err := inf.client.DefaultAPI.ApiAmbientApiServerV1WorkflowsGet(ctx).Execute()
	if err != nil {
		return err
	}

	currentIDs := make(map[string]bool)
	for _, workflow := range list.Items {
		id := workflow.GetId()
		currentIDs[id] = true

		if existing, found := inf.workflowCache[id]; found {
			if workflow.GetUpdatedAt() != existing.GetUpdatedAt() {
				inf.workflowCache[id] = workflow
				inf.dispatch(ctx, ResourceEvent{
					Type:      EventModified,
					Resource:  "workflows",
					Object:    workflow,
					OldObject: existing,
				})
			}
		} else {
			inf.workflowCache[id] = workflow
			inf.dispatch(ctx, ResourceEvent{
				Type:     EventAdded,
				Resource: "workflows",
				Object:   workflow,
			})
		}
	}

	for id, workflow := range inf.workflowCache {
		if !currentIDs[id] {
			delete(inf.workflowCache, id)
			inf.dispatch(ctx, ResourceEvent{
				Type:     EventDeleted,
				Resource: "workflows",
				Object:   workflow,
			})
		}
	}

	return nil
}

func (inf *Informer) syncTasks(ctx context.Context) error {
	list, _, err := inf.client.DefaultAPI.ApiAmbientApiServerV1TasksGet(ctx).Execute()
	if err != nil {
		return err
	}

	currentIDs := make(map[string]bool)
	for _, task := range list.Items {
		id := task.GetId()
		currentIDs[id] = true

		if existing, found := inf.taskCache[id]; found {
			if task.GetUpdatedAt() != existing.GetUpdatedAt() {
				inf.taskCache[id] = task
				inf.dispatch(ctx, ResourceEvent{
					Type:      EventModified,
					Resource:  "tasks",
					Object:    task,
					OldObject: existing,
				})
			}
		} else {
			inf.taskCache[id] = task
			inf.dispatch(ctx, ResourceEvent{
				Type:     EventAdded,
				Resource: "tasks",
				Object:   task,
			})
		}
	}

	for id, task := range inf.taskCache {
		if !currentIDs[id] {
			delete(inf.taskCache, id)
			inf.dispatch(ctx, ResourceEvent{
				Type:     EventDeleted,
				Resource: "tasks",
				Object:   task,
			})
		}
	}

	return nil
}

func (inf *Informer) dispatch(ctx context.Context, event ResourceEvent) {
	inf.mu.RLock()
	handlers := inf.handlers[event.Resource]
	inf.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			inf.logger.Error().
				Err(err).
				Str("resource", event.Resource).
				Str("event_type", string(event.Type)).
				Msg("handler failed")
		}
	}
}
