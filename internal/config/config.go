package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        int
	Env         string
	DatabaseURL string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        8080,
		Env:         getEnv("ENV", "development"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}

	if portStr := os.Getenv("PORT"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid PORT %q: %w", portStr, err)
		}
		cfg.Port = port
	}


	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
