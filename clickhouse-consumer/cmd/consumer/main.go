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
	audiov1 "github.com/agentjetson/contract/gen/go/audio/v1"
	detectionv1 "github.com/agentjetson/contract/gen/go/detection/v1"
	scenev1 "github.com/agentjetson/contract/gen/go/scene/v1"
	"github.com/agentjetson/contract/pkg/persistence"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

func decodeAlert(data []byte, seq uint64) ([]persistence.DetectionRow, error) {
	var a detectionv1.Alert
	if err := proto.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	dets := make([]mapx.DetectionFields, 0, len(a.GetDetections()))
	for _, d := range a.GetDetections() {
		df := mapx.DetectionFields{
			ClassID: d.GetClassId(), ClassName: d.GetClassName(),
			Confidence: d.GetConfidence(), TrackID: d.GetTrackId(),
		}
		if b := d.GetBox(); b != nil {
			df.X1, df.Y1, df.X2, df.Y2 = b.GetX1(), b.GetY1(), b.GetX2(), b.GetY2()
		}
		dets = append(dets, df)
	}
	labels := a.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	return mapx.AlertToRows(mapx.AlertFields{
		FrameID: a.GetFrameId(), Timestamp: a.GetTimestamp(), Source: a.GetSource(),
		WatchlistHit: a.GetWatchlistHit(), MatchedLabel: a.GetMatchedLabel(),
		E2ELatencyMs: a.GetE2ELatencyMs(), Labels: labels, Detections: dets,
	}, seq), nil
}

func decodeTranscript(data []byte, seq uint64) (persistence.TranscriptRow, error) {
	var t audiov1.Transcript
	if err := proto.Unmarshal(data, &t); err != nil {
		return persistence.TranscriptRow{}, err
	}
	labels := t.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	return mapx.TranscriptToRow(mapx.TranscriptFields{
		Timestamp: t.GetTimestamp(), AudioStart: t.GetAudioStart(), AudioEnd: t.GetAudioEnd(),
		Source: t.GetSource(), Text: t.GetText(), IsFinal: t.GetIsFinal(),
		Confidence: t.GetConfidence(), Language: t.GetLanguage(), SpeakerID: t.GetSpeakerId(),
		E2ELatencyMs: t.GetE2ELatencyMs(), Labels: labels,
	}, seq), nil
}

func decodeObject(data []byte, seq uint64) (persistence.ObjectRow, error) {
	var o detectionv1.ObjectEnvelope
	if err := proto.Unmarshal(data, &o); err != nil {
		return persistence.ObjectRow{}, err
	}
	labels := o.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	of := mapx.ObjectFields{
		FrameID: o.GetFrameId(), Timestamp: o.GetTimestamp(), Source: o.GetSource(),
		ClassName: o.GetClassName(), ClassID: o.GetClassId(), Confidence: o.GetConfidence(),
		TrackID: o.GetTrackId(), FrameWidth: o.GetFrameWidth(), FrameHeight: o.GetFrameHeight(),
		CaptureLatencyMs: o.GetCaptureLatencyMs(), Labels: labels,
	}
	if b := o.GetBox(); b != nil {
		of.X1, of.Y1, of.X2, of.Y2 = b.GetX1(), b.GetY1(), b.GetX2(), b.GetY2()
	}
	return mapx.ObjectToRow(of, seq), nil
}

func decodeResult(data []byte, seq uint64) (persistence.ResultRow, error) {
	var r detectionv1.CapabilityResult
	if err := proto.Unmarshal(data, &r); err != nil {
		return persistence.ResultRow{}, err
	}
	attrs := r.GetAttributes()
	if attrs == nil {
		attrs = map[string]string{}
	}
	labels := r.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	rf := mapx.ResultFields{
		FrameID: r.GetFrameId(), Timestamp: r.GetTimestamp(), Source: r.GetSource(),
		Capability: r.GetCapability(), TrackID: r.GetTrackId(), ClassName: r.GetClassName(),
		OCRText: r.GetOcrText(), OCRConfidence: r.GetOcrConfidence(),
		ProcessingMs: r.GetProcessingMs(), Attributes: attrs, Labels: labels,
	}
	if b := r.GetBox(); b != nil {
		rf.X1, rf.Y1, rf.X2, rf.Y2 = b.GetX1(), b.GetY1(), b.GetX2(), b.GetY2()
	}
	if pb := r.GetPlateBox(); pb != nil {
		rf.PlateX1, rf.PlateY1, rf.PlateX2, rf.PlateY2 = pb.GetX1(), pb.GetY1(), pb.GetX2(), pb.GetY2()
	}
	return mapx.ResultToRow(rf, seq), nil
}

func decodeScene(data []byte, seq uint64) (persistence.SceneRow, error) {
	var s scenev1.SceneResult
	if err := proto.Unmarshal(data, &s); err != nil {
		return persistence.SceneRow{}, err
	}
	return mapx.SceneToRow(mapx.SceneFields{
		FrameID: s.GetFrameId(), Timestamp: s.GetTimestamp(), Source: s.GetSource(),
		Level1: s.GetLevel1(), Level2: s.GetLevel2(),
		Level1Confidence: s.GetLevel1Confidence(), Level2Confidence: s.GetLevel2Confidence(),
		Specialists: s.GetSpecialists(), Backend: s.GetBackend(),
		TemporalRequested: s.GetTemporalRequested(), Notes: s.GetNotes(),
	}, seq), nil
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
		objBatch   []persistence.ObjectRow
		resBatch   []persistence.ResultRow
		scnBatch   []persistence.SceneRow
		detBatch   []persistence.DetectionRow
		trBatch    []persistence.TranscriptRow
		pendingAck []*nats.Msg
		lastFlush  = time.Now()
	)
	reserve := cfg.FlushSize * 2
	objBatch = make([]persistence.ObjectRow, 0, reserve)
	resBatch = make([]persistence.ResultRow, 0, reserve)
	scnBatch = make([]persistence.SceneRow, 0, reserve)
	detBatch = make([]persistence.DetectionRow, 0, reserve)
	trBatch = make([]persistence.TranscriptRow, 0, reserve)
	pendingAck = make([]*nats.Msg, 0, reserve*2)

	clearBatches := func() {
		objBatch = objBatch[:0]
		resBatch = resBatch[:0]
		scnBatch = scnBatch[:0]
		detBatch = detBatch[:0]
		trBatch = trBatch[:0]
		pendingAck = pendingAck[:0]
	}

	flush := func() {
		pending := len(objBatch) + len(resBatch) + len(scnBatch) + len(detBatch) + len(trBatch)
		if pending == 0 {
			return
		}
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

		failed := false
		for _, j := range jobs {
			if j.n == 0 {
				continue
			}
			if err := j.fn(); err != nil {
				slog.Error("insert", "table", j.name, "err", err, "n", j.n)
				failed = true
			} else {
				slog.Info("inserted", "table", j.name, "n", j.n)
			}
		}

		if failed {
			// Re-deliver so nothing is silently dropped.
			for _, m := range pendingAck {
				_ = m.Nak()
			}
			clearBatches()
			lastFlush = time.Now()
			return
		}
		for _, m := range pendingAck {
			_ = m.Ack()
		}
		clearBatches()
		lastFlush = time.Now()
	}

	handle := func(subject string, msg *nats.Msg) {
		seq := natsjs.StreamSeq(msg)
		switch {
		case subject == "cv.alert" || strings.HasPrefix(subject, "cv.alert"):
			rows, err := decodeAlert(msg.Data, seq)
			if err != nil {
				slog.Warn("decode Alert", "err", err, "bytes", len(msg.Data))
				_ = msg.Ack() // poison pill
				return
			}
			detBatch = append(detBatch, rows...)
			pendingAck = append(pendingAck, msg)

		case subject == "audio.transcript" || strings.HasPrefix(subject, "audio.transcript"):
			row, err := decodeTranscript(msg.Data, seq)
			if err != nil {
				slog.Warn("decode Transcript", "err", err)
				_ = msg.Ack()
				return
			}
			trBatch = append(trBatch, row)
			pendingAck = append(pendingAck, msg)

		case strings.HasPrefix(subject, "cv.object."):
			row, err := decodeObject(msg.Data, seq)
			if err != nil {
				slog.Warn("decode ObjectEnvelope", "err", err)
				_ = msg.Ack()
				return
			}
			objBatch = append(objBatch, row)
			pendingAck = append(pendingAck, msg)

		case strings.HasPrefix(subject, "cv.result."):
			row, err := decodeResult(msg.Data, seq)
			if err != nil {
				slog.Warn("decode CapabilityResult", "err", err)
				_ = msg.Ack()
				return
			}
			resBatch = append(resBatch, row)
			pendingAck = append(pendingAck, msg)

		case strings.HasPrefix(subject, "cv.scene."):
			row, err := decodeScene(msg.Data, seq)
			if err != nil {
				slog.Warn("decode SceneResult", "err", err)
				_ = msg.Ack()
				return
			}
			scnBatch = append(scnBatch, row)
			pendingAck = append(pendingAck, msg)

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
