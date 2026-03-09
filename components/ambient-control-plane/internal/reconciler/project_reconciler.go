package reconciler

import (
	"context"

	"github.com/ambient-code/platform/components/ambient-control-plane/internal/informer"
	"github.com/ambient-code/platform/components/ambient-control-plane/internal/kubeclient"
	"github.com/rs/zerolog"
)

type ProjectReconciler struct {
	factory *SDKClientFactory
	kube    *kubeclient.KubeClient
	logger  zerolog.Logger
}

func NewProjectReconciler(factory *SDKClientFactory, kube *kubeclient.KubeClient, logger zerolog.Logger) *ProjectReconciler {
	return &ProjectReconciler{
		factory: factory,
		kube:    kube,
		logger:  logger.With().Str("reconciler", "projects").Logger(),
	}
}

func (r *ProjectReconciler) Resource() string {
	return "projects"
}

func (r *ProjectReconciler) Reconcile(ctx context.Context, event informer.ResourceEvent) error {
	if event.Object.Project == nil {
		r.logger.Warn().Msg("expected project object in project event")
		return nil
	}
	project := *event.Object.Project

	r.logger.Info().
		Str("event", string(event.Type)).
		Str("project_id", project.ID).
		Str("name", project.Name).
		Msg("project event received - no action needed for MVP")

	return nil
}
