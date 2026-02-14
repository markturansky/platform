package reconciler

import (
	"context"
	"fmt"
	"strings"

	openapi "github.com/ambient/platform/components/ambient-api-server/pkg/api/openapi"
	"github.com/ambient/platform/components/ambient-control-plane/internal/informer"
	"github.com/ambient/platform/components/ambient-control-plane/internal/kubeclient"
	"github.com/rs/zerolog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Reconciler interface {
	Resource() string
	Reconcile(ctx context.Context, event informer.ResourceEvent) error
}

type FieldDiff struct {
	Field    string
	APIValue string
	K8sValue string
	Category string
}

type SessionReconciler struct {
	client *openapi.APIClient
	kube   *kubeclient.KubeClient
	logger zerolog.Logger
}

func NewSessionReconciler(client *openapi.APIClient, kube *kubeclient.KubeClient, logger zerolog.Logger) *SessionReconciler {
	return &SessionReconciler{
		client: client,
		kube:   kube,
		logger: logger.With().Str("reconciler", "sessions").Logger(),
	}
}

func (r *SessionReconciler) Resource() string {
	return "sessions"
}

func (r *SessionReconciler) Reconcile(ctx context.Context, event informer.ResourceEvent) error {
	session, ok := event.Object.(openapi.Session)
	if !ok {
		r.logger.Warn().
			Str("actual_type", fmt.Sprintf("%T", event.Object)).
			Msg("type assertion failed: expected openapi.Session")
		return nil
	}

	r.logger.Info().
		Str("event", string(event.Type)).
		Str("session_id", session.GetId()).
		Str("name", session.GetName()).
		Msg("session event received")

	if event.Type == informer.EventDeleted {
		r.logger.Info().
			Str("session_id", session.GetId()).
			Msg("session deleted from API, skipping k8s lookup")
		return nil
	}

	diffs := r.diffAgainstKubernetes(ctx, session)
	if len(diffs) == 0 {
		r.logger.Debug().
			Str("session_id", session.GetId()).
			Msg("no field differences detected between API and k8s")
		return nil
	}

	r.logFieldDiffs(session.GetId(), session.GetName(), diffs)
	return nil
}

func (r *SessionReconciler) diffAgainstKubernetes(ctx context.Context, session openapi.Session) []FieldDiff {
	sessionName := session.GetName()
	if sessionName == "" {
		r.logger.Warn().
			Str("session_id", session.GetId()).
			Msg("session has no name, cannot look up k8s resource")
		return nil
	}

	cr, err := r.kube.GetAgenticSession(ctx, sessionName)
	if err != nil {
		r.logger.Warn().
			Err(err).
			Str("session_name", sessionName).
			Str("namespace", r.kube.Namespace()).
			Msg("k8s lookup failed for session")
		return nil
	}

	return r.compareSessionToCR(session, cr)
}

func (r *SessionReconciler) compareSessionToCR(session openapi.Session, cr *unstructured.Unstructured) []FieldDiff {
	var diffs []FieldDiff

	crName := cr.GetName()
	if apiName := session.GetName(); apiName != crName {
		diffs = append(diffs, FieldDiff{
			Field:    "name",
			APIValue: apiName,
			K8sValue: crName,
			Category: "identity",
		})
	}

	displayName, _, _ := unstructured.NestedString(cr.Object, "spec", "displayName")
	if apiName := session.GetName(); apiName != displayName {
		diffs = append(diffs, FieldDiff{
			Field:    "name↔displayName",
			APIValue: fmt.Sprintf("name=%q", apiName),
			K8sValue: fmt.Sprintf("spec.displayName=%q", displayName),
			Category: "field-mapping",
		})
	}

	crPrompt, _, _ := unstructured.NestedString(cr.Object, "spec", "initialPrompt")
	apiPrompt := ptrStr(session.Prompt)
	if apiPrompt != crPrompt {
		diffs = append(diffs, FieldDiff{
			Field:    "prompt↔initialPrompt",
			APIValue: truncate(apiPrompt, 80),
			K8sValue: truncate(crPrompt, 80),
			Category: "field-mapping",
		})
	}

	apiRepoURL := ptrStr(session.RepoUrl)
	crRepos, _, _ := unstructured.NestedSlice(cr.Object, "spec", "repos")
	crRepoURLs := extractRepoURLs(crRepos)
	if apiRepoURL != "" && !containsString(crRepoURLs, apiRepoURL) {
		diffs = append(diffs, FieldDiff{
			Field:    "repo_url↔repos",
			APIValue: fmt.Sprintf("repo_url=%q", apiRepoURL),
			K8sValue: fmt.Sprintf("spec.repos=%v", crRepoURLs),
			Category: "structural",
		})
	} else if apiRepoURL == "" && len(crRepoURLs) > 0 {
		diffs = append(diffs, FieldDiff{
			Field:    "repo_url↔repos",
			APIValue: "(empty)",
			K8sValue: fmt.Sprintf("spec.repos=%v", crRepoURLs),
			Category: "structural",
		})
	}

	if session.WorkflowId != nil {
		crWorkflow, crWfFound, _ := unstructured.NestedMap(cr.Object, "spec", "activeWorkflow")
		if !crWfFound {
			diffs = append(diffs, FieldDiff{
				Field:    "workflow_id↔activeWorkflow",
				APIValue: fmt.Sprintf("workflow_id=%q", ptrStr(session.WorkflowId)),
				K8sValue: "(not set)",
				Category: "field-mapping",
			})
		} else {
			crGitURL, _ := crWorkflow["gitUrl"].(string)
			diffs = append(diffs, FieldDiff{
				Field:    "workflow_id↔activeWorkflow",
				APIValue: fmt.Sprintf("workflow_id=%q", ptrStr(session.WorkflowId)),
				K8sValue: fmt.Sprintf("spec.activeWorkflow.gitUrl=%q", crGitURL),
				Category: "field-mapping",
			})
		}
	}

	diffs = append(diffs, r.findAPIOnlyFields(session)...)
	diffs = append(diffs, r.findK8sOnlyFields(cr)...)

	return diffs
}

func (r *SessionReconciler) findAPIOnlyFields(session openapi.Session) []FieldDiff {
	var diffs []FieldDiff

	if session.CreatedByUserId != nil && *session.CreatedByUserId != "" {
		diffs = append(diffs, FieldDiff{
			Field:    "created_by_user_id",
			APIValue: *session.CreatedByUserId,
			K8sValue: "(no equivalent — k8s uses spec.userContext.userId)",
			Category: "api-only",
		})
	}

	if session.AssignedUserId != nil && *session.AssignedUserId != "" {
		diffs = append(diffs, FieldDiff{
			Field:    "assigned_user_id",
			APIValue: *session.AssignedUserId,
			K8sValue: "(no equivalent field in CRD)",
			Category: "api-only",
		})
	}

	if session.Id != nil && *session.Id != "" {
		diffs = append(diffs, FieldDiff{
			Field:    "id",
			APIValue: *session.Id,
			K8sValue: "(k8s uses metadata.uid)",
			Category: "identity-mapping",
		})
	}

	return diffs
}

func (r *SessionReconciler) findK8sOnlyFields(cr *unstructured.Unstructured) []FieldDiff {
	var diffs []FieldDiff

	if _, found, _ := unstructured.NestedBool(cr.Object, "spec", "interactive"); found {
		diffs = append(diffs, FieldDiff{
			Field:    "spec.interactive",
			APIValue: "(no equivalent field in API)",
			K8sValue: "present",
			Category: "k8s-only",
		})
	}

	if _, found, _ := unstructured.NestedMap(cr.Object, "spec", "llmSettings"); found {
		diffs = append(diffs, FieldDiff{
			Field:    "spec.llmSettings",
			APIValue: "(no equivalent field in API)",
			K8sValue: "present (model, temperature, maxTokens)",
			Category: "k8s-only",
		})
	}

	if v, found, _ := unstructured.NestedInt64(cr.Object, "spec", "timeout"); found {
		diffs = append(diffs, FieldDiff{
			Field:    "spec.timeout",
			APIValue: "(no equivalent field in API)",
			K8sValue: fmt.Sprintf("%d", v),
			Category: "k8s-only",
		})
	}

	if _, found, _ := unstructured.NestedMap(cr.Object, "spec", "userContext"); found {
		diffs = append(diffs, FieldDiff{
			Field:    "spec.userContext",
			APIValue: "(no equivalent — API has created_by_user_id)",
			K8sValue: "present (userId, displayName, groups)",
			Category: "k8s-only",
		})
	}

	if _, found, _ := unstructured.NestedMap(cr.Object, "spec", "resourceOverrides"); found {
		diffs = append(diffs, FieldDiff{
			Field:    "spec.resourceOverrides",
			APIValue: "(no equivalent field in API)",
			K8sValue: "present (cpu, memory, storageClass, priorityClass)",
			Category: "k8s-only",
		})
	}

	if _, found, _ := unstructured.NestedStringMap(cr.Object, "spec", "environmentVariables"); found {
		diffs = append(diffs, FieldDiff{
			Field:    "spec.environmentVariables",
			APIValue: "(no equivalent field in API)",
			K8sValue: "present",
			Category: "k8s-only",
		})
	}

	if _, found, _ := unstructured.NestedString(cr.Object, "spec", "project"); found {
		diffs = append(diffs, FieldDiff{
			Field:    "spec.project",
			APIValue: "(no equivalent field in API)",
			K8sValue: "present",
			Category: "k8s-only",
		})
	}

	if _, found, _ := unstructured.NestedMap(cr.Object, "status"); found {
		diffs = append(diffs, FieldDiff{
			Field:    "status",
			APIValue: "(no status fields in API Session)",
			K8sValue: "present (phase, conditions, reconciledRepos, etc.)",
			Category: "k8s-only",
		})
	}

	return diffs
}

func (r *SessionReconciler) logFieldDiffs(sessionID, sessionName string, diffs []FieldDiff) {
	categories := map[string][]FieldDiff{}
	for _, d := range diffs {
		categories[d.Category] = append(categories[d.Category], d)
	}

	r.logger.Warn().
		Str("session_id", sessionID).
		Str("session_name", sessionName).
		Int("total_diffs", len(diffs)).
		Msg("API↔K8s field differences detected")

	for cat, catDiffs := range categories {
		for _, d := range catDiffs {
			r.logger.Info().
				Str("session_id", sessionID).
				Str("category", cat).
				Str("field", d.Field).
				Str("api_value", d.APIValue).
				Str("k8s_value", d.K8sValue).
				Msg("field diff")
		}
	}
}

type WorkflowReconciler struct {
	client *openapi.APIClient
	kube   *kubeclient.KubeClient
	logger zerolog.Logger
}

func NewWorkflowReconciler(client *openapi.APIClient, kube *kubeclient.KubeClient, logger zerolog.Logger) *WorkflowReconciler {
	return &WorkflowReconciler{
		client: client,
		kube:   kube,
		logger: logger.With().Str("reconciler", "workflows").Logger(),
	}
}

func (r *WorkflowReconciler) Resource() string {
	return "workflows"
}

func (r *WorkflowReconciler) Reconcile(ctx context.Context, event informer.ResourceEvent) error {
	workflow, ok := event.Object.(openapi.Workflow)
	if !ok {
		r.logger.Warn().
			Str("actual_type", fmt.Sprintf("%T", event.Object)).
			Msg("type assertion failed: expected openapi.Workflow")
		return nil
	}

	r.logger.Info().
		Str("event", string(event.Type)).
		Str("workflow_id", workflow.GetId()).
		Str("name", workflow.GetName()).
		Msg("workflow event received (no k8s CRD — database-only resource)")

	return nil
}

type TaskReconciler struct {
	client *openapi.APIClient
	kube   *kubeclient.KubeClient
	logger zerolog.Logger
}

func NewTaskReconciler(client *openapi.APIClient, kube *kubeclient.KubeClient, logger zerolog.Logger) *TaskReconciler {
	return &TaskReconciler{
		client: client,
		kube:   kube,
		logger: logger.With().Str("reconciler", "tasks").Logger(),
	}
}

func (r *TaskReconciler) Resource() string {
	return "tasks"
}

func (r *TaskReconciler) Reconcile(ctx context.Context, event informer.ResourceEvent) error {
	task, ok := event.Object.(openapi.Task)
	if !ok {
		r.logger.Warn().
			Str("actual_type", fmt.Sprintf("%T", event.Object)).
			Msg("type assertion failed: expected openapi.Task")
		return nil
	}

	r.logger.Info().
		Str("event", string(event.Type)).
		Str("task_id", task.GetId()).
		Str("name", task.GetName()).
		Msg("task event received (no k8s CRD — database-only resource)")

	return nil
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func extractRepoURLs(repos []interface{}) []string {
	var urls []string
	for _, r := range repos {
		if m, ok := r.(map[string]interface{}); ok {
			if u, ok := m["url"].(string); ok {
				urls = append(urls, u)
			}
		}
	}
	return urls
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
