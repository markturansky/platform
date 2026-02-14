package kubeclient

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var AgenticSessionGVR = schema.GroupVersionResource{
	Group:    "vteam.ambient-code",
	Version:  "v1alpha1",
	Resource: "agenticsessions",
}

type KubeClient struct {
	dynamic   dynamic.Interface
	namespace string
	logger    zerolog.Logger
}

func New(kubeconfig string, namespace string, logger zerolog.Logger) (*KubeClient, error) {
	cfg, err := buildRestConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("building kubeconfig: %w", err)
	}

	cfg.QPS = 50
	cfg.Burst = 100

	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	kc := &KubeClient{
		dynamic:   dynClient,
		namespace: namespace,
		logger:    logger.With().Str("component", "kubeclient").Logger(),
	}

	kc.logger.Info().
		Str("namespace", namespace).
		Msg("kubernetes client initialized")

	return kc, nil
}

func buildRestConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if envPath := os.Getenv("KUBECONFIG"); envPath != "" {
		return clientcmd.BuildConfigFromFlags("", envPath)
	}

	home, _ := os.UserHomeDir()
	localPath := home + "/.kube/config"
	if _, err := os.Stat(localPath); err == nil {
		return clientcmd.BuildConfigFromFlags("", localPath)
	}

	return rest.InClusterConfig()
}

func (kc *KubeClient) GetAgenticSession(ctx context.Context, name string) (*unstructured.Unstructured, error) {
	return kc.dynamic.Resource(AgenticSessionGVR).Namespace(kc.namespace).Get(ctx, name, metav1.GetOptions{})
}

func (kc *KubeClient) ListAgenticSessions(ctx context.Context) (*unstructured.UnstructuredList, error) {
	return kc.dynamic.Resource(AgenticSessionGVR).Namespace(kc.namespace).List(ctx, metav1.ListOptions{})
}

func (kc *KubeClient) Namespace() string {
	return kc.namespace
}
