package config

import (
	"os"
	"strconv"
	"time"
)

// Config is loaded from environment.
// Combined ingest owns NATS; PUBLISHER_ADDR is no longer required.
type Config struct {
	GRPCAddr   string // listen address for IngestService (+ NatsPublisherService)
	NATSURL    string
	ClientName string
}

func Load() Config {
	return Config{
		GRPCAddr:   getenv("GRPC_ADDR", "0.0.0.0:50052"),
		NATSURL:    getenv("NATS_URL", "nats://localhost:4222"),
		ClientName: getenv("NATS_CLIENT_NAME", "ingest"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

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
