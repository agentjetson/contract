package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime configuration loaded from environment variables.
type Config struct {
	// Server
	HTTPAddr string // e.g. ":8080"
	DemoMode bool   // if true, serve in-memory sample data (no CH required)

	// ClickHouse
	ClickHouseHost     string
	ClickHousePort     int
	ClickHouseUser     string
	ClickHousePassword string
	ClickHouseDatabase string
	ApplySchema        bool // create 002 tables if missing

	// NATS (optional live path)
	NATSURL            string
	NATSEnableLive     bool
	NATSJetStreamStream string

	// Defaults
	DefaultTimeWindowSec int
	DefaultLimit         int
}

func Load() *Config {
	return &Config{
		HTTPAddr:             getEnv("HTTP_ADDR", ":8080"),
		DemoMode:             getEnvBool("DEMO_MODE", false),
		ClickHouseHost:       getEnv("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort:       getEnvInt("CLICKHOUSE_PORT", 9000),
		ClickHouseUser:       getEnv("CLICKHOUSE_USER", "default"),
		ClickHousePassword:   getEnv("CLICKHOUSE_PASSWORD", "pass"),
		ClickHouseDatabase:   getEnv("CLICKHOUSE_DATABASE", "default"),
		ApplySchema:          getEnvBool("APPLY_SCHEMA", true),
		NATSURL:              getEnv("NATS_URL", "nats://localhost:4222"),
		NATSEnableLive:       getEnvBool("NATS_ENABLE_LIVE", false),
		NATSJetStreamStream:  getEnv("NATS_JS_STREAM", "CV"),
		DefaultTimeWindowSec: getEnvInt("DEFAULT_TIME_WINDOW_SEC", 30),
		DefaultLimit:         getEnvInt("DEFAULT_LIMIT", 5),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return fallback
}

// ClickHouseDSN returns a native protocol DSN.
func (c *Config) ClickHouseDSN() string {
	return "clickhouse://" + c.ClickHouseUser + ":" + c.ClickHousePassword +
		"@" + c.ClickHouseHost + ":" + strconv.Itoa(c.ClickHousePort) +
		"/" + c.ClickHouseDatabase + "?dial_timeout=5s&max_execution_time=60"
}

// DefaultTimeout for tool handlers.
func DefaultTimeout() time.Duration {
	return 10 * time.Second
}
