package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	openapi "github.com/ambient/platform/components/ambient-api-server/pkg/api/openapi"
	"github.com/ambient/platform/components/ambient-control-plane/internal/config"
	"github.com/ambient/platform/components/ambient-control-plane/internal/informer"
	"github.com/ambient/platform/components/ambient-control-plane/internal/kubeclient"
	"github.com/ambient/platform/components/ambient-control-plane/internal/reconciler"
	"github.com/rs/zerolog"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := setupLogger(cfg.LogLevel)
	logger.Info().
		Str("version", version).
		Str("build_time", buildTime).
		Str("api_server", cfg.APIServerURL).
		Str("namespace", cfg.Namespace).
		Dur("poll_interval", cfg.PollInterval).
		Int("workers", cfg.WorkerCount).
		Msg("starting ambient-control-plane")

	apiClient := buildAPIClient(cfg)

	kube, err := kubeclient.New(cfg.Kubeconfig, cfg.Namespace, logger)
	if err != nil {
		return fmt.Errorf("initializing kubernetes client: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	inf := informer.New(apiClient, cfg.PollInterval, logger)

	sessionReconciler := reconciler.NewSessionReconciler(apiClient, kube, logger)
	workflowReconciler := reconciler.NewWorkflowReconciler(apiClient, kube, logger)
	taskReconciler := reconciler.NewTaskReconciler(apiClient, kube, logger)

	registerReconciler(inf, sessionReconciler)
	registerReconciler(inf, workflowReconciler)
	registerReconciler(inf, taskReconciler)

	logger.Info().Msg("all reconcilers registered, entering run loop")

	err = inf.Run(ctx)
	if err != nil && ctx.Err() != nil {
		logger.Info().Msg("shutdown complete")
		return nil
	}
	return err
}

func buildAPIClient(cfg *config.ControlPlaneConfig) *openapi.APIClient {
	apiCfg := openapi.NewConfiguration()
	apiCfg.Servers = openapi.ServerConfigurations{
		{URL: cfg.APIServerURL, Description: "ambient-api-server"},
	}
	apiCfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}

	if cfg.APIToken != "" {
		apiCfg.AddDefaultHeader("Authorization", "Bearer "+cfg.APIToken)
	}

	return openapi.NewAPIClient(apiCfg)
}

func registerReconciler(inf *informer.Informer, rec reconciler.Reconciler) {
	inf.RegisterHandler(rec.Resource(), rec.Reconcile)
}

func setupLogger(level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	return zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		With().
		Timestamp().
		Str("service", "ambient-control-plane").
		Logger().
		Level(lvl)
}
