package config

import (
	"os"
	"strconv"
)

type Config struct {
	DataDir      string
	BaseURL      string
	SyncInterval int
	Port         string
}

func Load() *Config {
	c := &Config{
		DataDir:      envOr("FRONTPAGE_DATA_DIR", "./testdata"),
		BaseURL:      envOr("FRONTPAGE_BASE_URL", "http://localhost:8080"),
		SyncInterval: envOrInt("FRONTPAGE_SYNC_INTERVAL", 900),
		Port:         envOr("FRONTPAGE_PORT", "8080"),
	}
	return c
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
