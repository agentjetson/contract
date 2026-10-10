// scene-gate: per-camera decision gate (profile + audio intent).
//
// Listens on audio.transcript; emits SceneResult via ingest when a high-confidence
// intent or fixed-role profile fires. Otherwise forwards to visual scene-router
// (edge still owns frames).
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/protobuf/proto"

	audiov1 "github.com/agentjetson/core/gen/go/audio/v1"
	"github.com/agentjetson/core/scene-gate/internal/config"
	"github.com/agentjetson/core/scene-gate/internal/decision"
	"github.com/agentjetson/core/scene-gate/internal/natsjs"
	"github.com/agentjetson/core/scene-gate/internal/profile"
	"github.com/agentjetson/core/scene-gate/internal/publish"
	"github.com/agentjetson/core/scene-gate/internal/taxonomy"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	sourceID := flag.String("source-id", "", "bring-up: simulate OnFrame for this source_id")
	simulateAudio := flag.String("simulate-audio", "", "bring-up: simulate OnAudio transcript")
	flag.Parse()

	cfg := config.Load()

	tax, err := taxonomy.Load(cfg.TaxonomyPath)
	if err != nil {
		// Fallback for local bring-up from repo root
		tax, err = taxonomy.Load("domain/taxonomy.yaml")
		if err != nil {
			tax, err = taxonomy.Load("../domain/taxonomy.yaml")
		}
	}
	if err != nil {
		slog.Error("taxonomy", "err", err)
		os.Exit(1)
	}

	store, err := profile.Load(cfg.CameraProfilesPath)
	if err != nil {
		store, err = profile.Load("domain/camera_profiles.yaml")
		if err != nil {
			store, err = profile.Load("../domain/camera_profiles.yaml")
		}
	}
	if err != nil {
		slog.Error("profiles", "err", err)
		os.Exit(1)
	}

	engine := decision.New(tax, store)

	// Bring-up path: no NATS required
	if *sourceID != "" {
		runBringUp(engine, *sourceID, *simulateAudio)
		return
	}

	var publisher *publish.IngestClient
	if cfg.PublishViaIngest {
		publisher, err = publish.DialIngest(cfg.IngestAddr)
		if err != nil {
			slog.Warn("ingest dial failed — will log decisions only", "addr", cfg.IngestAddr, "err", err)
		} else {
			defer publisher.Close()
			slog.Info("ingest connected", "addr", cfg.IngestAddr)
		}
	}

	nc, js, err := natsjs.Connect(cfg.NATSURL, "cv-scene-gate")
	if err != nil {
		slog.Error("nats", "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	if err := natsjs.WaitStream(js, cfg.StreamAudio, 60*time.Second); err != nil {
		slog.Warn("audio stream not ready — waiting in loop", "stream", cfg.StreamAudio, "err", err)
	}

	audioSub, err := natsjs.EnsurePull(js, cfg.StreamAudio, "scene-gate-audio", cfg.AudioSubject)
	if err != nil {
		slog.Error("subscribe audio", "err", err)
		os.Exit(1)
	}
	defer audioSub.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("scene-gate ready",
		"nats", cfg.NATSURL,
		"audio_subject", cfg.AudioSubject,
		"ingest", cfg.IngestAddr,
	)

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown complete")
			return
		default:
			msgs, err := audioSub.Fetch(cfg.FetchBatch, cfg.FetchTimeout)
			if err != nil {
				slog.Warn("fetch", "err", err)
				continue
			}
			for _, msg := range msgs {
				handleTranscript(ctx, engine, publisher, msg.Data)
				_ = msg.Ack()
			}
		}
	}
}

func handleTranscript(ctx context.Context, engine *decision.Engine, publisher *publish.IngestClient, data []byte) {
	var t audiov1.Transcript
	if err := proto.Unmarshal(data, &t); err != nil {
		slog.Warn("unmarshal transcript", "err", err)
		return
	}
	if !t.GetIsFinal() && t.GetText() == "" {
		return
	}
	source := t.GetSource()
	if source == "" {
		source = "unknown"
	}
	r := engine.OnAudio(source, t.GetText())
	slog.Info("decision",
		"action", r.Action.String(),
		"source", source,
		"l2", r.L2,
		"reason", r.Reason,
	)
	if r.Action == decision.EmitNow && publisher != nil {
		if err := publisher.PublishScene(ctx, r); err != nil {
			slog.Error("publish scene", "err", err)
		}
	}
}

func runBringUp(engine *decision.Engine, sourceID, simulateAudio string) {
	var r decision.Result
	if simulateAudio != "" {
		r = engine.OnAudio(sourceID, simulateAudio)
	} else {
		r = engine.OnFrame(sourceID)
	}
	slog.Info("bring-up decision",
		"action", r.Action.String(),
		"source", r.SourceID,
		"l1", r.L1,
		"l2", r.L2,
		"confidence", r.Confidence,
		"backend", r.Backend,
		"specialists", r.Specialists,
		"skip_visual", r.SkipVisual,
		"reason", r.Reason,
	)
}
