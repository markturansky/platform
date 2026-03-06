package k8sinformer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ambient-code/platform/components/ambient-control-plane/internal/kubeclient"
	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
)

// K8sEvent represents a Kubernetes resource event
type K8sEvent struct {
	Type      K8sEventType
	Object    *unstructured.Unstructured
	OldObject *unstructured.Unstructured // Only populated for UPDATE events
}

type K8sEventType string

const (
	K8sEventAdded    K8sEventType = "ADDED"
	K8sEventModified K8sEventType = "MODIFIED"
	K8sEventDeleted  K8sEventType = "DELETED"
)

// K8sEventHandler handles Kubernetes resource events
type K8sEventHandler func(ctx context.Context, event K8sEvent) error

// K8sInformer watches Kubernetes resources and dispatches events to registered handlers
type K8sInformer struct {
	kube            *kubeclient.KubeClient
	logger          zerolog.Logger
	handlers        []K8sEventHandler
	mu              sync.RWMutex
	resourceVersion string
	lastSyncTime    time.Time
	eventQueue      chan K8sEvent
	resourceCache   map[string]*unstructured.Unstructured // key: namespace/name
}

// NewK8sInformer creates a new Kubernetes informer for AgenticSession resources
func NewK8sInformer(kube *kubeclient.KubeClient, logger zerolog.Logger) *K8sInformer {
	return &K8sInformer{
		kube:          kube,
		logger:        logger.With().Str("component", "k8s-informer").Logger(),
		eventQueue:    make(chan K8sEvent, 256),
		resourceCache: make(map[string]*unstructured.Unstructured),
	}
}

// RegisterHandler adds an event handler
func (ki *K8sInformer) RegisterHandler(handler K8sEventHandler) {
	ki.mu.Lock()
	defer ki.mu.Unlock()
	ki.handlers = append(ki.handlers, handler)
}

// Run starts the informer and blocks until context is cancelled
func (ki *K8sInformer) Run(ctx context.Context) error {
	ki.logger.Info().Msg("starting Kubernetes informer for AgenticSession resources")

	// Start event processing goroutine
	go ki.processEvents(ctx)

	// Initial sync
	if err := ki.initialSync(ctx); err != nil {
		return fmt.Errorf("initial sync failed: %w", err)
	}

	// Start watch loop
	return ki.watchLoop(ctx)
}

// initialSync performs an initial list of all AgenticSession resources
func (ki *K8sInformer) initialSync(ctx context.Context) error {
	ki.logger.Info().Msg("performing initial sync of AgenticSession resources")

	// List all AgenticSessions across all namespaces
	// Note: This requires cluster-wide permissions for AgenticSession resources
	list, err := ki.kube.GetDynamicClient().Resource(kubeclient.AgenticSessionGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing AgenticSession resources: %w", err)
	}

	ki.mu.Lock()
	ki.resourceVersion = list.GetResourceVersion()
	ki.lastSyncTime = time.Now()

	// Populate initial cache and send ADDED events
	for _, item := range list.Items {
		key := ki.resourceKey(&item)
		ki.resourceCache[key] = item.DeepCopy()

		// Send ADDED event for initial sync
		select {
		case ki.eventQueue <- K8sEvent{
			Type:   K8sEventAdded,
			Object: item.DeepCopy(),
		}:
		case <-ctx.Done():
			ki.mu.Unlock()
			return ctx.Err()
		default:
			ki.logger.Warn().Str("resource", key).Msg("event queue full during initial sync, dropping event")
		}
	}
	ki.mu.Unlock()

	ki.logger.Info().
		Int("count", len(list.Items)).
		Str("resource_version", ki.resourceVersion).
		Msg("initial sync completed")

	return nil
}

// watchLoop maintains a watch on AgenticSession resources
func (ki *K8sInformer) watchLoop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			ki.logger.Info().Msg("Kubernetes informer stopping due to context cancellation")
			return ctx.Err()
		default:
		}

		if err := ki.runSingleWatch(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			ki.logger.Warn().Err(err).Msg("watch failed, retrying after backoff")

			// Exponential backoff with jitter
			backoff := time.Second * 5
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
		}
	}
}

// runSingleWatch runs a single watch session
func (ki *K8sInformer) runSingleWatch(ctx context.Context) error {
	ki.mu.RLock()
	resourceVersion := ki.resourceVersion
	ki.mu.RUnlock()

	ki.logger.Debug().
		Str("resource_version", resourceVersion).
		Msg("starting watch session")

	watcher, err := ki.kube.WatchAgenticSessions(ctx, resourceVersion)
	if err != nil {
		return fmt.Errorf("creating watch: %w", err)
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				ki.logger.Debug().Msg("watch channel closed")
				return fmt.Errorf("watch channel closed")
			}

			if err := ki.handleWatchEvent(ctx, event); err != nil {
				ki.logger.Warn().Err(err).Str("event_type", string(event.Type)).Msg("failed to handle watch event")
				continue
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// handleWatchEvent processes a single watch event
func (ki *K8sInformer) handleWatchEvent(ctx context.Context, event watch.Event) error {
	obj, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected object type: %T", event.Object)
	}

	key := ki.resourceKey(obj)

	ki.mu.Lock()
	// Update resource version from the latest event
	ki.resourceVersion = obj.GetResourceVersion()

	var k8sEvent K8sEvent
	var oldObj *unstructured.Unstructured

	switch event.Type {
	case watch.Added:
		ki.resourceCache[key] = obj.DeepCopy()
		k8sEvent = K8sEvent{
			Type:   K8sEventAdded,
			Object: obj.DeepCopy(),
		}

	case watch.Modified:
		oldObj = ki.resourceCache[key] // may be nil if we missed the initial add
		ki.resourceCache[key] = obj.DeepCopy()
		k8sEvent = K8sEvent{
			Type:      K8sEventModified,
			Object:    obj.DeepCopy(),
			OldObject: oldObj,
		}

	case watch.Deleted:
		oldObj = ki.resourceCache[key]
		delete(ki.resourceCache, key)
		k8sEvent = K8sEvent{
			Type:      K8sEventDeleted,
			Object:    obj.DeepCopy(),
			OldObject: oldObj,
		}

	case watch.Error:
		ki.mu.Unlock()
		return fmt.Errorf("watch error event: %v", event.Object)

	default:
		ki.mu.Unlock()
		ki.logger.Warn().Str("event_type", string(event.Type)).Msg("unknown watch event type")
		return nil
	}
	ki.mu.Unlock()

	// Send event to queue for processing
	select {
	case ki.eventQueue <- k8sEvent:
	case <-ctx.Done():
		return ctx.Err()
	default:
		ki.logger.Warn().
			Str("resource", key).
			Str("event_type", string(k8sEvent.Type)).
			Msg("event queue full, dropping event")
	}

	return nil
}

// processEvents processes events from the queue and dispatches to handlers
func (ki *K8sInformer) processEvents(ctx context.Context) {
	ki.logger.Info().Msg("starting event processor")

	for {
		select {
		case event := <-ki.eventQueue:
			ki.dispatchEvent(ctx, event)
		case <-ctx.Done():
			ki.logger.Info().Msg("event processor stopping")
			return
		}
	}
}

// dispatchEvent sends an event to all registered handlers
func (ki *K8sInformer) dispatchEvent(ctx context.Context, event K8sEvent) {
	ki.mu.RLock()
	handlers := make([]K8sEventHandler, len(ki.handlers))
	copy(handlers, ki.handlers)
	ki.mu.RUnlock()

	resourceKey := ki.resourceKey(event.Object)

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			ki.logger.Warn().
				Err(err).
				Str("resource", resourceKey).
				Str("event_type", string(event.Type)).
				Msg("handler failed to process event")
		}
	}
}

// resourceKey generates a unique key for a resource
func (ki *K8sInformer) resourceKey(obj *unstructured.Unstructured) string {
	namespace := obj.GetNamespace()
	if namespace == "" {
		return obj.GetName()
	}
	return namespace + "/" + obj.GetName()
}

// StatusFromCR extracts status information from an AgenticSession CR
func StatusFromCR(cr *unstructured.Unstructured) (phase string, uid string, namespace string, found bool) {
	if cr == nil {
		return "", "", "", false
	}

	// Extract phase from status
	if statusMap, statusFound, _ := unstructured.NestedMap(cr.Object, "status"); statusFound && statusMap != nil {
		if phaseVal, phaseFound, _ := unstructured.NestedString(cr.Object, "status", "phase"); phaseFound {
			phase = strings.TrimSpace(phaseVal)
		}
	}

	uid = string(cr.GetUID())
	namespace = cr.GetNamespace()
	found = uid != "" && namespace != ""

	return phase, uid, namespace, found
}
