package config

import (
	"os"
	"strconv"
	"time"
)

// Config is loaded from environment (same vars as the C++ binary).
type Config struct {
	NATSURL    string
	GRPCAddr   string
	ClientName string
}

func Load() Config {
	return Config{
		NATSURL:    getenv("NATS_URL", "nats://localhost:4222"),
		GRPCAddr:   getenv("GRPC_ADDR", "0.0.0.0:50051"),
		ClientName: getenv("NATS_CLIENT_NAME", "edge-nats-publisher"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// helpers kept for future use
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
