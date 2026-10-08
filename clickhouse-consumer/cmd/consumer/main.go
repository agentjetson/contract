package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/agentjetson/contract/clickhouse-consumer/internal/config"
	mapx "github.com/agentjetson/contract/clickhouse-consumer/internal/map"
	"github.com/agentjetson/contract/clickhouse-consumer/internal/natsjs"
	"github.com/agentjetson/contract/pkg/persistence"
	"github.com/nats-io/nats.go"
)

// Decode hooks — replace body with generated protobuf Unmarshal once
// contract gen/go is available (make generate):
//
//	detectionv1 "github.com/agentjetson/contract/gen/go/detection/v1"
//	audiov1     "github.com/agentjetson/contract/gen/go/audio/v1"
//	scenev1     "github.com/agentjetson/contract/gen/go/scene/v1"
//
//	var a detectionv1.Alert
//	if err := proto.Unmarshal(data, &a); err != nil { ... }
//	return mapx.AlertToRows(mapx.AlertFields{... from a ...}, seq), nil

func decodeAlert(data []byte, seq uint64) ([]persistence.DetectionRow, error) {
	_ = data
	// TODO: wire gen/go detection.v1.Alert
	return mapx.AlertToRows(mapx.AlertFields{
		FrameID: 0, Source: "unknown", ClassID: -1,
	}, seq), errNeedGen
}

func decodeTranscript(data []byte, seq uint64) (persistence.TranscriptRow, error) {
	_ = data
	return mapx.TranscriptToRow(mapx.TranscriptFields{Source: "unknown", IsFinal: true}, seq), errNeedGen
}

func decodeObject(data []byte, seq uint64) (persistence.ObjectRow, error) {
	_ = data
	return mapx.ObjectToRow(mapx.ObjectFields{Source: "unknown"}, seq), errNeedGen
}

func decodeResult(data []byte, seq uint64) (persistence.ResultRow, error) {
	_ = data
	return mapx.ResultToRow(mapx.ResultFields{Source: "unknown", Capability: "unknown"}, seq), errNeedGen
}

func decodeScene(data []byte, seq uint64) (persistence.SceneRow, error) {
	_ = data
	return mapx.SceneToRow(mapx.SceneFields{Source: "unknown", Level1: "unknown"}, seq), errNeedGen
}

var errNeedGen = errNeedGeneratedProtos{}

type errNeedGeneratedProtos struct{}

func (errNeedGeneratedProtos) Error() string {
	return "protobuf decode requires contract gen/go (run make generate and replace decode* hooks)"
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg := config.Load()

	ch, err := persistence.Open(cfg.CH)
	if err != nil {
		slog.Error("clickhouse", "err", err)
		os.Exit(1)
	}
	defer ch.Close()
	slog.Info("clickhouse connected", "host", cfg.CH.Host, "port", cfg.CH.Port, "db", cfg.CH.Database)

	nc, js, err := natsjs.Connect(cfg.NATSURL)
	if err != nil {
		slog.Error("nats", "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	for _, s := range []string{cfg.StreamEvents, cfg.StreamAlerts, cfg.StreamAudio} {
		if err := natsjs.WaitStream(js, s, 60*time.Second); err != nil {
			slog.Warn("stream not ready, will retry on pull", "stream", s, "err", err)
		}
	}

	type subDef struct {
		stream, durable, subject string
		required                 bool
	}
	defs := []subDef{
		{cfg.StreamEvents, "ch-consumer-objects", "cv.object.>", false},
		{cfg.StreamEvents, "ch-consumer-results", "cv.result.>", false},
		{cfg.StreamEvents, "ch-consumer-scenes", "cv.scene.>", false},
		{cfg.StreamAlerts, "ch-consumer-alerts", "cv.alert", true},
		{cfg.StreamAudio, "ch-consumer-audio", "audio.transcript", false},
	}

	subs := make(map[string]*natsjs.Sub)
	for _, d := range defs {
		sub, err := natsjs.EnsurePull(js, d.stream, d.durable, d.subject)
		if err != nil {
			if d.required {
				slog.Error("required subscription failed", "subject", d.subject, "err", err)
				os.Exit(1)
			}
			slog.Warn("subscription unavailable", "subject", d.subject, "err", err)
			continue
		}
		subs[d.subject] = sub
		slog.Info("subscribed", "subject", d.subject, "durable", d.durable)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var (
		objBatch  []persistence.ObjectRow
		resBatch  []persistence.ResultRow
		scnBatch  []persistence.SceneRow
		detBatch  []persistence.DetectionRow
		trBatch   []persistence.TranscriptRow
		lastFlush = time.Now()
	)
	reserve := cfg.FlushSize * 2
	objBatch = make([]persistence.ObjectRow, 0, reserve)
	resBatch = make([]persistence.ResultRow, 0, reserve)
	scnBatch = make([]persistence.SceneRow, 0, reserve)
	detBatch = make([]persistence.DetectionRow, 0, reserve)
	trBatch = make([]persistence.TranscriptRow, 0, reserve)

	flush := func() {
		fctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		type job struct {
			name string
			n    int
			fn   func() error
		}
		jobs := []job{
			{"cv_objects", len(objBatch), func() error { return ch.InsertObjects(fctx, objBatch) }},
			{"cv_results", len(resBatch), func() error { return ch.InsertResults(fctx, resBatch) }},
			{"cv_scenes", len(scnBatch), func() error { return ch.InsertScenes(fctx, scnBatch) }},
			{"cv_detections", len(detBatch), func() error { return ch.InsertDetections(fctx, detBatch) }},
			{"audio_transcripts", len(trBatch), func() error { return ch.InsertTranscripts(fctx, trBatch) }},
		}
		for _, j := range jobs {
			if j.n == 0 {
				continue
			}
			if err := j.fn(); err != nil {
				slog.Error("insert", "table", j.name, "err", err, "n", j.n)
			} else {
				slog.Info("inserted", "table", j.name, "n", j.n)
			}
		}
		objBatch = objBatch[:0]
		resBatch = resBatch[:0]
		scnBatch = scnBatch[:0]
		detBatch = detBatch[:0]
		trBatch = trBatch[:0]
		lastFlush = time.Now()
	}

	handle := func(subject string, msg *nats.Msg) {
		seq := natsjs.StreamSeq(msg)
		switch {
		case subject == "cv.alert" || strings.HasPrefix(subject, "cv.alert"):
			rows, err := decodeAlert(msg.Data, seq)
			if err != nil {
				slog.Warn("decode Alert", "err", err, "bytes", len(msg.Data))
			} else {
				detBatch = append(detBatch, rows...)
			}
			_ = msg.Ack()

		case subject == "audio.transcript" || strings.HasPrefix(subject, "audio.transcript"):
			row, err := decodeTranscript(msg.Data, seq)
			if err != nil {
				slog.Warn("decode Transcript", "err", err)
			} else {
				trBatch = append(trBatch, row)
			}
			_ = msg.Ack()

		case strings.HasPrefix(subject, "cv.object."):
			row, err := decodeObject(msg.Data, seq)
			if err != nil {
				slog.Warn("decode ObjectEnvelope", "err", err)
			} else {
				objBatch = append(objBatch, row)
			}
			_ = msg.Ack()

		case strings.HasPrefix(subject, "cv.result."):
			row, err := decodeResult(msg.Data, seq)
			if err != nil {
				slog.Warn("decode CapabilityResult", "err", err)
			} else {
				resBatch = append(resBatch, row)
			}
			_ = msg.Ack()

		case strings.HasPrefix(subject, "cv.scene."):
			row, err := decodeScene(msg.Data, seq)
			if err != nil {
				slog.Warn("decode SceneResult", "err", err)
			} else {
				scnBatch = append(scnBatch, row)
			}
			_ = msg.Ack()

		default:
			slog.Debug("unhandled subject", "subject", subject)
			_ = msg.Ack()
		}
	}

	slog.Info("clickhouse-consumer ready",
		"subjects", mapKeys(subs),
		"flush_size", cfg.FlushSize,
		"flush_ms", cfg.FlushInterval.Milliseconds(),
	)
	slog.Warn("protobuf decode stubs active — run make generate and fill decode* in main.go before production")

	for {
		select {
		case <-ctx.Done():
			flush()
			for _, s := range subs {
				s.Close()
			}
			slog.Info("shutdown complete")
			return
		default:
		}

		for subject, sub := range subs {
			msgs, err := sub.Fetch(cfg.FetchBatch, cfg.FetchTimeout)
			if err != nil {
				slog.Warn("fetch", "subject", subject, "err", err)
				continue
			}
			for _, m := range msgs {
				handle(m.Subject, m)
			}
		}

		pending := len(objBatch) + len(resBatch) + len(scnBatch) + len(detBatch) + len(trBatch)
		sizeFlush := len(objBatch) >= cfg.FlushSize ||
			len(resBatch) >= cfg.FlushSize ||
			len(scnBatch) >= cfg.FlushSize ||
			len(detBatch) >= cfg.FlushSize ||
			len(trBatch) >= cfg.FlushSize
		timeFlush := time.Since(lastFlush) > cfg.FlushInterval && pending > 0
		if sizeFlush || timeFlush {
			flush()
		}
	}
}

func mapKeys(m map[string]*natsjs.Sub) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
