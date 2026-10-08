package config

import (
	"os"
	"strconv"
	"time"

	"github.com/agentjetson/contract/pkg/persistence"
)

// Config is the clickhouse-consumer runtime configuration.
type Config struct {
	NATSURL string

	CH persistence.Config

	FlushInterval time.Duration
	FlushSize     int
	FetchBatch    int
	FetchTimeout  time.Duration

	// Stream / durable names — match domain/nats-subjects.yaml
	StreamEvents string // CV_EVENTS
	StreamAlerts string // CV_ALERTS
	StreamAudio  string // AUDIO_EVENTS
}

func Load() Config {
	return Config{
		NATSURL: getenv("NATS_URL", "nats://localhost:4222"),
		CH: persistence.Config{
			Host:     getenv("CLICKHOUSE_HOST", "localhost"),
			Port:     getenvInt("CLICKHOUSE_PORT", 9000),
			User:     getenv("CLICKHOUSE_USER", "default"),
			Password: getenv("CLICKHOUSE_PASSWORD", "pass"),
			Database: getenv("CLICKHOUSE_DB", "default"),
		},
		FlushInterval: time.Duration(getenvInt("FLUSH_MS", 500)) * time.Millisecond,
		FlushSize:     getenvInt("FLUSH_SIZE", 32),
		FetchBatch:    getenvInt("FETCH_BATCH", 8),
		FetchTimeout:  time.Duration(getenvInt("FETCH_TIMEOUT_MS", 100)) * time.Millisecond,
		StreamEvents:  getenv("STREAM_EVENTS", "CV_EVENTS"),
		StreamAlerts:  getenv("STREAM_ALERTS", "CV_ALERTS"),
		StreamAudio:   getenv("STREAM_AUDIO", "AUDIO_EVENTS"),
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
