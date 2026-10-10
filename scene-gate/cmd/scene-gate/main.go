// scene-gate: per-camera decision gate (profile + audio intent + dynamic sources).
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
	"github.com/agentjetson/core/scene-gate/internal/registry"
	"github.com/agentjetson/core/scene-gate/internal/sources"
	"github.com/agentjetson/core/scene-gate/internal/taxonomy"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	sourceID := flag.String("source-id", "", "bring-up: simulate OnFrame for this source_id")
	simulateAudio := flag.String("simulate-audio", "", "bring-up: simulate OnAudio transcript")
	emitProfilesOnly := flag.Bool("emit-profiles", false, "bring-up: emit profile scenes for all registered sources and exit")
	flag.Parse()

	cfg := config.Load()

	tax, err := taxonomy.Load(cfg.TaxonomyPath)
	if err != nil {
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

	if *sourceID != "" {
		runBringUp(engine, *sourceID, *simulateAudio)
		return
	}

	registered, err := loadSources(cfg.CameraSourcesPath)
	if err != nil {
		slog.Error("camera sources", "err", err)
		os.Exit(1)
	}

	if *emitProfilesOnly {
		emitRegisteredProfiles(context.Background(), engine, nil, registered)
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

	var sourceSub *natsjs.Sub
	live := registry.New(cfg.SourceDebounce)
	if cfg.DynamicSources {
		_ = natsjs.WaitStream(js, cfg.StreamEvents, 30*time.Second)
		natsjs.EnsureSourceSubjects(js, cfg.StreamEvents)
		sourceSub, err = natsjs.EnsurePull(js, cfg.StreamEvents, "scene-gate-source", cfg.SourceSubject)
		if err != nil {
			slog.Warn("subscribe source lifecycle failed — static sources only",
				"subject", cfg.SourceSubject, "err", err)
		} else {
			defer sourceSub.Close()
			slog.Info("dynamic sources enabled", "subject", cfg.SourceSubject, "debounce", cfg.SourceDebounce.String())
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("scene-gate ready",
		"nats", cfg.NATSURL,
		"audio_subject", cfg.AudioSubject,
		"ingest", cfg.IngestAddr,
		"emit_profiles", cfg.EmitProfiles,
		"static_sources", len(registered),
		"dynamic_sources", cfg.DynamicSources && sourceSub != nil,
		"profile_refresh", cfg.ProfileRefresh.String(),
	)

	if cfg.EmitProfiles && len(registered) > 0 {
		emitRegisteredProfiles(ctx, engine, publisher, registered)
	}

	var refresh <-chan time.Time
	if cfg.EmitProfiles && cfg.ProfileRefresh > 0 && len(registered) > 0 {
		t := time.NewTicker(cfg.ProfileRefresh)
		defer t.Stop()
		refresh = t.C
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown complete", "live_sources", live.Count())
			return
		case <-refresh:
			emitRegisteredProfiles(ctx, engine, publisher, registered)
		default:
			if sourceSub != nil {
				msgs, err := sourceSub.Fetch(cfg.FetchBatch, cfg.FetchTimeout)
				if err != nil {
					slog.Warn("fetch source", "err", err)
				} else {
					for _, msg := range msgs {
						handleSourceEvent(ctx, engine, publisher, live, msg.Data, msg.Subject)
						_ = msg.Ack()
					}
				}
			}
			msgs, err := audioSub.Fetch(cfg.FetchBatch, cfg.FetchTimeout)
			if err != nil {
				slog.Warn("fetch audio", "err", err)
				continue
			}
			for _, msg := range msgs {
				handleTranscript(ctx, engine, publisher, msg.Data)
				_ = msg.Ack()
			}
		}
	}
}

func loadSources(path string) ([]sources.Entry, error) {
	entries, err := sources.Load(path)
	if err != nil {
		return nil, err
	}
	if len(entries) > 0 {
		return entries, nil
	}
	for _, p := range []string{"domain/camera_sources.yaml", "../domain/camera_sources.yaml"} {
		entries, err = sources.Load(p)
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			return entries, nil
		}
	}
	return entries, nil
}

func emitRegisteredProfiles(ctx context.Context, engine *decision.Engine, publisher *publish.IngestClient, registered []sources.Entry) {
	emitted := 0
	for _, e := range registered {
		if emitProfileForSource(ctx, engine, publisher, e.ID) {
			emitted++
		}
	}
	slog.Info("profile emit pass done", "registered", len(registered), "emitted", emitted)
}

func emitProfileForSource(ctx context.Context, engine *decision.Engine, publisher *publish.IngestClient, id string) bool {
	r := engine.OnFrame(id)
	slog.Info("profile decision",
		"source", id,
		"action", r.Action.String(),
		"l1", r.L1,
		"l2", r.L2,
		"backend", r.Backend,
		"specialists", r.Specialists,
		"reason", r.Reason,
	)
	if r.Action != decision.EmitNow {
		return false
	}
	if publisher != nil {
		if err := publisher.PublishScene(ctx, r); err != nil {
			slog.Error("publish profile scene", "source", id, "err", err)
			return false
		}
	}
	return true
}

func handleSourceEvent(ctx context.Context, engine *decision.Engine, publisher *publish.IngestClient, live *registry.Live, data []byte, subject string) {
	ev, err := registry.ParseJSON(data)
	if err != nil {
		// Infer event from subject token when payload is minimal
		slog.Debug("source event json", "err", err, "subject", subject)
		return
	}
	if ev.Event == "" && subject != "" {
		// cv.source.up → up
		parts := splitSubject(subject)
		if len(parts) >= 3 {
			ev.Event = parts[2]
			ev.normalize()
		}
	}
	id := ev.ID()
	if id == "" {
		slog.Warn("source event missing source_id", "subject", subject)
		return
	}
	if ev.IsDown() {
		live.Remove(id)
		slog.Info("source down", "source", id)
		return
	}
	if !ev.IsUp() {
		slog.Debug("source event ignored", "event", ev.Event, "source", id)
		return
	}
	if !live.Touch(id, time.Now()) {
		slog.Debug("source heartbeat debounced", "source", id)
		return
	}
	slog.Info("source up / emit", "source", id, "event", ev.Event)
	_ = emitProfileForSource(ctx, engine, publisher, id)
}

func splitSubject(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
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
