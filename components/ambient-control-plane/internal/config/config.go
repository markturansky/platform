package config

import (
	"fmt"
	"os"
	"strings"
)

type ControlPlaneConfig struct {
	APIServerURL   string
	APIToken       string
	GRPCServerAddr string
	GRPCUseTLS     bool
	LogLevel       string
	Kubeconfig     string
	Mode           string
	Reconcilers    []string
}

func Load() (*ControlPlaneConfig, error) {
	cfg := &ControlPlaneConfig{
		APIServerURL:   envOrDefault("AMBIENT_API_SERVER_URL", "http://localhost:8000"),
		APIToken:       os.Getenv("AMBIENT_API_TOKEN"),
		GRPCServerAddr: envOrDefault("AMBIENT_GRPC_SERVER_ADDR", "localhost:8001"),
		GRPCUseTLS:     os.Getenv("AMBIENT_GRPC_USE_TLS") == "true",
		LogLevel:       envOrDefault("LOG_LEVEL", "info"),
		Kubeconfig:     os.Getenv("KUBECONFIG"),
		Mode:           envOrDefault("MODE", "kube"),
		Reconcilers:    parseReconcilers(envOrDefault("RECONCILERS", "tally")),
	}

	if cfg.APIToken == "" {
		return nil, fmt.Errorf("AMBIENT_API_TOKEN environment variable is required")
	}

	switch cfg.Mode {
	case "kube", "test":
	default:
		return nil, fmt.Errorf("unknown MODE %q: must be one of kube, test", cfg.Mode)
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseReconcilers(reconcilersStr string) []string {
	if reconcilersStr == "" {
		return []string{"tally"}
	}

	reconcilers := strings.Split(reconcilersStr, ",")
	var result []string
	for _, r := range reconcilers {
		r = strings.TrimSpace(r)
		if r != "" {
			result = append(result, r)
		}
	}

	if len(result) == 0 {
		return []string{"tally"}
	}

	return result
}
