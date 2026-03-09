package reconciler

import (
	"context"
	"time"

	"github.com/ambient-code/platform/components/ambient-control-plane/internal/informer"
	"github.com/ambient-code/platform/components/ambient-control-plane/internal/kubeclient"
	"github.com/ambient-code/platform/components/ambient-sdk/go-sdk/types"
	"github.com/rs/zerolog"
)

type SimpleKubeReconciler struct {
	factory *SDKClientFactory
	kube    *kubeclient.KubeClient
	logger  zerolog.Logger
}

func NewKubeReconciler(factory *SDKClientFactory, kube *kubeclient.KubeClient, logger zerolog.Logger) *SimpleKubeReconciler {
	return &SimpleKubeReconciler{
		factory: factory,
		kube:    kube,
		logger:  logger.With().Str("reconciler", "kube-direct").Logger(),
	}
}

func (r *SimpleKubeReconciler) Resource() string {
	return "sessions"
}

func (r *SimpleKubeReconciler) Reconcile(ctx context.Context, event informer.ResourceEvent) error {
	if event.Object.Session == nil {
		r.logger.Warn().Msg("expected session object in session event")
		return nil
	}
	session := *event.Object.Session

	r.logger.Info().
		Str("event", string(event.Type)).
		Str("session_id", session.ID).
		Str("name", session.Name).
		Msg("kube direct reconciler: session event received")

	switch event.Type {
	case informer.EventAdded:
		return r.handleAdded(ctx, session)
	case informer.EventModified:
		return r.handleModified(ctx, session)
	case informer.EventDeleted:
		return r.handleDeleted(ctx, session)
	default:
		return nil
	}
}

func (r *SimpleKubeReconciler) handleAdded(ctx context.Context, session types.Session) error {
	if session.Phase == PhasePending || session.Phase == "" {
		return r.logSessionCreation(ctx, session)
	}
	return nil
}

func (r *SimpleKubeReconciler) handleModified(ctx context.Context, session types.Session) error {
	switch session.Phase {
	case PhasePending:
		return r.logSessionCreation(ctx, session)
	case PhaseStopping:
		return r.logSessionStopping(ctx, session)
	case PhaseRunning:
		return r.logSessionRunning(ctx, session)
	}
	return nil
}

func (r *SimpleKubeReconciler) handleDeleted(ctx context.Context, session types.Session) error {
	return r.logSessionDeletion(ctx, session)
}

func (r *SimpleKubeReconciler) logSessionCreation(ctx context.Context, session types.Session) error {
	namespace := namespaceForSession(session)

	r.logger.Info().
		Str("session_id", session.ID).
		Str("namespace", namespace).
		Msg("kube reconciler: session pending — kubernetes job creation not yet implemented")

	return nil
}

func (r *SimpleKubeReconciler) logSessionStopping(ctx context.Context, session types.Session) error {
	r.logger.Info().
		Str("session_id", session.ID).
		Msg("DIRECT KUBE RECONCILER: would stop kubernetes resources for session")

	r.updateSessionPhase(ctx, session, PhaseStopped)
	return nil
}

func (r *SimpleKubeReconciler) logSessionRunning(ctx context.Context, session types.Session) error {
	r.logger.Debug().
		Str("session_id", session.ID).
		Msg("DIRECT KUBE RECONCILER: monitoring running session")
	return nil
}

func (r *SimpleKubeReconciler) logSessionDeletion(ctx context.Context, session types.Session) error {
	r.logger.Info().
		Str("session_id", session.ID).
		Msg("DIRECT KUBE RECONCILER: would cleanup kubernetes resources for session")
	return nil
}

func (r *SimpleKubeReconciler) updateSessionPhase(ctx context.Context, session types.Session, newPhase string) {
	if session.Phase == newPhase {
		return
	}

	if session.ProjectID == "" {
		r.logger.Debug().Str("session_id", session.ID).Msg("skipping phase update: session has no project_id")
		return
	}

	sdk, err := r.factory.ForProject(session.ProjectID)
	if err != nil {
		r.logger.Warn().Err(err).Str("session_id", session.ID).Msg("failed to get SDK client for phase update")
		return
	}

	patch := map[string]interface{}{
		"phase": newPhase,
	}

	if newPhase == PhaseRunning && session.StartTime == nil {
		now := time.Now()
		patch["start_time"] = &now
	}

	if (newPhase == PhaseCompleted || newPhase == PhaseFailed || newPhase == PhaseStopped) && session.CompletionTime == nil {
		now := time.Now()
		patch["completion_time"] = &now
	}

	if _, err := sdk.Sessions().UpdateStatus(ctx, session.ID, patch); err != nil {
		r.logger.Warn().Err(err).Str("session_id", session.ID).Str("phase", newPhase).Msg("failed to update session phase")
		return
	}

	r.logger.Info().
		Str("session_id", session.ID).
		Str("old_phase", session.Phase).
		Str("new_phase", newPhase).
		Msg("DIRECT KUBE RECONCILER: updated session phase")
}
