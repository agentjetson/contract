package config

import (
	"os"
	"strconv"
	"time"
)

// Config is the aggregator runtime configuration.
type Config struct {
	NATSURL string

	StreamEvents string // CV_EVENTS (objects + results)
	StreamAlerts string // CV_ALERTS (cv.alert publish target stream ownership)

	FetchBatch   int
	FetchTimeout time.Duration

	// Correlation window: emit without ALPR after this, prune after this.
	EmitTimeout  time.Duration // default 800ms
	PruneTimeout time.Duration // default 5s
	PruneEvery   time.Duration // default 1s

	// Watchlist entries as "label:min_conf" pairs (comma-separated).
	// Empty → built-in defaults (person/car/truck + sample plate).
	WatchlistRaw string
}

func Load() Config {
	return Config{
		NATSURL:      getenv("NATS_URL", "nats://localhost:4222"),
		StreamEvents: getenv("STREAM_EVENTS", "CV_EVENTS"),
		StreamAlerts: getenv("STREAM_ALERTS", "CV_ALERTS"),
		FetchBatch:   getenvInt("FETCH_BATCH", 8),
		FetchTimeout: time.Duration(getenvInt("FETCH_TIMEOUT_MS", 200)) * time.Millisecond,
		EmitTimeout:  time.Duration(getenvInt("EMIT_TIMEOUT_MS", 800)) * time.Millisecond,
		PruneTimeout: time.Duration(getenvInt("PRUNE_TIMEOUT_MS", 5000)) * time.Millisecond,
		PruneEvery:   time.Duration(getenvInt("PRUNE_EVERY_MS", 1000)) * time.Millisecond,
		WatchlistRaw: getenv("WATCHLIST", ""),
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
