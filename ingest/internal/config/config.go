package config

import (
	"os"
	"strconv"
	"time"
)

// Config is loaded from environment (same vars as the C++ ingest_server).
type Config struct {
	GRPCAddr      string // listen address for IngestService
	PublisherAddr string // dial address for NatsPublisherService
}

func Load() Config {
	return Config{
		GRPCAddr:      getenv("GRPC_ADDR", "0.0.0.0:50052"),
		PublisherAddr: getenv("PUBLISHER_ADDR", "localhost:50051"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// helpers kept for symmetry with sibling services
func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
