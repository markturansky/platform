package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type ControlPlaneConfig struct {
	APIServerURL string
	APIToken     string
	PollInterval time.Duration
	WorkerCount  int
	LogLevel     string
	Kubeconfig   string
	Namespace    string
}

func Load() (*ControlPlaneConfig, error) {
	cfg := &ControlPlaneConfig{
		APIServerURL: envOrDefault("AMBIENT_API_SERVER_URL", "http://localhost:8000"),
		APIToken:     os.Getenv("AMBIENT_API_TOKEN"),
		LogLevel:     envOrDefault("LOG_LEVEL", "info"),
		Kubeconfig:   os.Getenv("KUBECONFIG"),
		Namespace:    envOrDefault("NAMESPACE", "ambient-code"),
	}

	pollSeconds, err := strconv.Atoi(envOrDefault("POLL_INTERVAL_SECONDS", "5"))
	if err != nil {
		return nil, fmt.Errorf("invalid POLL_INTERVAL_SECONDS: %w", err)
	}
	cfg.PollInterval = time.Duration(pollSeconds) * time.Second

	workers, err := strconv.Atoi(envOrDefault("WORKER_COUNT", "2"))
	if err != nil {
		return nil, fmt.Errorf("invalid WORKER_COUNT: %w", err)
	}
	cfg.WorkerCount = workers

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
