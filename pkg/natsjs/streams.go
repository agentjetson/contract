package natsjs

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// StreamDef matches domain/nats-subjects.yaml stream ownership.
type StreamDef struct {
	Name     string
	Subjects []string
	MaxAge   time.Duration // 0 → 1h default
}

// DefaultStreams returns the canonical JetStream layout from domain/nats-subjects.yaml.
func DefaultStreams() []StreamDef {
	return []StreamDef{
		{
			Name:     "CV_EVENTS",
			Subjects: []string{"cv.object.>", "cv.result.>", "cv.scene.>"},
			MaxAge:   time.Hour,
		},
		{
			Name:     "CV_ALERTS",
			Subjects: []string{"cv.alert"},
			MaxAge:   time.Hour,
		},
		{
			Name:     "AUDIO_EVENTS",
			Subjects: []string{"audio.transcript"},
			MaxAge:   time.Hour,
		},
	}
}

// EnsureStream creates the stream if missing, or updates subjects when possible.
// On subject-overlap / update conflicts it logs a warning and continues (stream
// already exists and is usable) — same behaviour as the C++ nats_publisher.
func EnsureStream(js nats.JetStreamContext, def StreamDef) error {
	maxAge := def.MaxAge
	if maxAge == 0 {
		maxAge = time.Hour
	}
	cfg := &nats.StreamConfig{
		Name:      def.Name,
		Subjects:  def.Subjects,
		Retention: nats.LimitsPolicy,
		Storage:   nats.FileStorage,
		MaxAge:    maxAge,
		MaxMsgs:   -1,
		MaxBytes:  -1,
	}

	info, err := js.AddStream(cfg)
	if err == nil {
		slog.Info("stream created", "stream", def.Name, "subjects", def.Subjects)
		_ = info
		return nil
	}

	// Stream already exists — try update (subject expansion).
	if isStreamExists(err) {
		slog.Info("stream already exists – updating subjects if possible", "stream", def.Name)
		if _, uerr := js.UpdateStream(cfg); uerr != nil {
			slog.Warn("UpdateStream failed – keeping existing config",
				"stream", def.Name, "err", uerr)
			// Non-fatal: stream exists and is usable.
			return nil
		}
		slog.Info("stream updated", "stream", def.Name, "subjects", def.Subjects)
		return nil
	}

	return fmt.Errorf("AddStream %s: %w", def.Name, err)
}

// EnsureAllStreams ensures every stream in DefaultStreams (or the provided list).
// AUDIO_EVENTS is non-fatal when ensure fails (another owner may already hold it).
func EnsureAllStreams(js nats.JetStreamContext, defs []StreamDef) error {
	if len(defs) == 0 {
		defs = DefaultStreams()
	}
	for _, d := range defs {
		if err := EnsureStream(js, d); err != nil {
			if d.Name == "AUDIO_EVENTS" {
				slog.Warn("AUDIO_EVENTS ensure non-fatal", "err", err)
				continue
			}
			return err
		}
	}
	return nil
}

func isStreamExists(err error) bool {
	// nats.go returns ErrStreamNameAlreadyInUse or a JetStream API error with code 10058.
	if err == nil {
		return false
	}
	if err == nats.ErrStreamNameAlreadyInUse {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "stream name already in use") ||
		strings.Contains(msg, "already exists")
}
