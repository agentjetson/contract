package config

import (
	"os"
	"strconv"
	"strings"
)

// Config holds runtime settings for the object-storage service.
type Config struct {
	GRPCAddr string

	// Backend: "minio" | "s3" | "filesystem"
	Backend string

	// S3 / MinIO
	Endpoint       string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
	Region         string
	ForcePathStyle bool

	// Local filesystem fallback (dev / DEMO_MODE)
	FSRoot string

	// ClickHouse metadata writer (optional).
	// Prefer discrete env vars (same names as clickhouse-consumer / Makefile).
	// CLICKHOUSE_DSN is accepted for docs/compose compatibility but not required
	// when Host is set.
	ClickHouseDSN     string
	ClickHouseEnabled bool
	ClickHouseHost    string
	ClickHousePort    int
	ClickHouseUser    string
	ClickHousePassword string
	ClickHouseDB      string

	// Demo mode skips external dependencies
	DemoMode bool
}

func FromEnv() Config {
	c := Config{
		GRPCAddr:           envOr("GRPC_ADDR", "0.0.0.0:50055"),
		Backend:            envOr("STORAGE_BACKEND", "minio"),
		Endpoint:           envOr("S3_ENDPOINT", "localhost:9000"),
		AccessKey:          envOr("S3_ACCESS_KEY", "minioadmin"),
		SecretKey:          envOr("S3_SECRET_KEY", "minioadmin"),
		Bucket:             envOr("S3_BUCKET", "agentjetson"),
		UseSSL:             envBool("S3_USE_SSL", false),
		Region:             envOr("S3_REGION", "us-east-1"),
		ForcePathStyle:     envBool("S3_FORCE_PATH_STYLE", true),
		FSRoot:             envOr("FS_ROOT", "./data"),
		ClickHouseDSN:      envOr("CLICKHOUSE_DSN", "clickhouse://default:pass@localhost:9000/default"),
		ClickHouseEnabled:  envBool("CLICKHOUSE_ENABLED", false),
		ClickHouseHost:     envOr("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort:     envInt("CLICKHOUSE_PORT", 9000),
		ClickHouseUser:     envOr("CLICKHOUSE_USER", "default"),
		ClickHousePassword: envOr("CLICKHOUSE_PASSWORD", "pass"),
		ClickHouseDB:       envOr("CLICKHOUSE_DB", envOr("CLICKHOUSE_DATABASE", "default")),
		DemoMode:           envBool("DEMO_MODE", true),
	}
	if c.DemoMode {
		c.Backend = "filesystem"
		c.ClickHouseEnabled = false
	}
	return c
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
