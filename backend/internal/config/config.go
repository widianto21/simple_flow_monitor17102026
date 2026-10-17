package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	Port      string // HTTP server port
	DBPath    string // SQLite database file path
	CORSAllow string // Allowed CORS origin for the frontend dev server

	// IngestAPIKey, when non-empty, is required in the X-API-Key header of
	// every /api/ingest/* request. Empty disables authentication.
	IngestAPIKey string

	// AutoRegister creates placeholder flows/nodes for unknown flowKey/nodeKey
	// in incoming events, so instrumentation never loses telemetry.
	AutoRegister bool

	// DefaultSLAMinutes is used for nodes without their own timeoutMinutes.
	DefaultSLAMinutes int

	// StuckCheckIntervalSeconds controls the background stuck/alerts watcher.
	StuckCheckIntervalSeconds int

	// SeedDemo loads demo flows + simulated history when the database is empty.
	SeedDemo bool
}

// Load reads configuration from the environment with sensible defaults.
func Load() *Config {
	return &Config{
		Port:                      getEnv("FLOWMON_PORT", "8080"),
		DBPath:                    getEnv("FLOWMON_DB", "flowmon.db"),
		CORSAllow:                 getEnv("FLOWMON_CORS_ALLOW", "http://localhost:5173"),
		IngestAPIKey:              getEnv("FLOWMON_INGEST_KEY", ""),
		AutoRegister:              getBoolEnv("FLOWMON_AUTO_REGISTER", true),
		DefaultSLAMinutes:         getIntEnv("FLOWMON_DEFAULT_SLA_MINUTES", 60),
		StuckCheckIntervalSeconds: getIntEnv("FLOWMON_STUCK_INTERVAL_SECONDS", 30),
		SeedDemo:                  getBoolEnv("FLOWMON_SEED_DEMO", true),
	}
}

func getEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getBoolEnv(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getIntEnv(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}
