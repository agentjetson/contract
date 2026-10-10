package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime settings for scene-gate.
type Config struct {
	NATSURL            string
	IngestAddr         string
	TaxonomyPath       string
	CameraProfilesPath string
	CameraSourcesPath  string
	AudioSubject       string
	StreamAudio        string
	StreamEvents       string
	FetchBatch         int
	FetchTimeout       time.Duration
	PublishViaIngest   bool // true = gRPC IngestScene; false = direct NATS (future)
	EmitProfiles       bool // emit backend=profile SceneResult for registered sources
	ProfileRefresh     time.Duration // 0 = startup only; >0 re-emit on this interval
}

func Load() Config {
	refreshMin := envInt("PROFILE_REFRESH_MIN", 0)
	return Config{
		NATSURL:            env("NATS_URL", "nats://localhost:4222"),
		IngestAddr:         env("INGEST_ADDR", "localhost:50052"),
		TaxonomyPath:       env("TAXONOMY_PATH", "/config/taxonomy.yaml"),
		CameraProfilesPath: env("CAMERA_PROFILES_PATH", "/config/camera_profiles.yaml"),
		CameraSourcesPath:  env("CAMERA_SOURCES_PATH", "/config/camera_sources.yaml"),
		AudioSubject:       env("AUDIO_SUBJECT", "audio.transcript"),
		StreamAudio:        env("STREAM_AUDIO", "AUDIO_EVENTS"),
		StreamEvents:       env("STREAM_EVENTS", "CV_EVENTS"),
		FetchBatch:         envInt("FETCH_BATCH", 8),
		FetchTimeout:       time.Duration(envInt("FETCH_TIMEOUT_MS", 200)) * time.Millisecond,
		PublishViaIngest:   envBool("PUBLISH_VIA_INGEST", true),
		EmitProfiles:       envBool("EMIT_PROFILES", true),
		ProfileRefresh:     time.Duration(refreshMin) * time.Minute,
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}
