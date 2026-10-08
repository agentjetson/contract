// Aggregator: correlates ObjectEnvelopes + CapabilityResults by
// (frame_id, track_id), applies watchlist, emits final Alert to cv.alert.
//
// Go port of core/src/aggregator/main.cpp. Policy / watchlist / evidence
// assembly lives here — out of the recorder and out of individual specialists.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/agentjetson/contract/aggregator/internal/config"
	"github.com/agentjetson/contract/aggregator/internal/correlate"
	"github.com/agentjetson/contract/aggregator/internal/natsjs"
	"github.com/nats-io/nats.go"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg := config.Load()

	nc, js, err := natsjs.Connect(cfg.NATSURL, "cv-aggregator")
	if err != nil {
		slog.Error("nats", "err", err)
		os.Exit(1)
	}
	defer nc.Close()

	if err := natsjs.WaitStream(js, cfg.StreamEvents, 60*time.Second); err != nil {
		slog.Error("stream", "name", cfg.StreamEvents, "err", err)
		os.Exit(1)
	}
	// Alerts stream is owned by nats_publisher; we only publish.
	if err := natsjs.WaitStream(js, cfg.StreamAlerts, 30*time.Second); err != nil {
		slog.Warn("stream alerts not ready yet — publishes may fail until nats-publisher starts",
			"stream", cfg.StreamAlerts, "err", err)
	}

	objSub, err := natsjs.EnsurePull(js, cfg.StreamEvents, "agg-objects", "cv.object.>")
	if err != nil {
		slog.Error("subscribe objects", "err", err)
		os.Exit(1)
	}
	defer objSub.Close()

	resSub, err := natsjs.EnsurePull(js, cfg.StreamEvents, "agg-results", "cv.result.>")
	if err != nil {
		slog.Error("subscribe results", "err", err)
		os.Exit(1)
	}
	defer resSub.Close()

	publish := func(subject string, data []byte) error {
		_, err := js.Publish(subject, data)
		return err
	}

	engine := correlate.New(
		correlate.ParseWatchlist(cfg.WatchlistRaw),
		cfg.EmitTimeout,
		cfg.PruneTimeout,
		publish,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	slog.Info("aggregator ready",
		"nats", cfg.NATSURL,
		"stream", cfg.StreamEvents,
		"durables", []string{"agg-objects", "agg-results"},
		"emit_timeout_ms", cfg.EmitTimeout.Milliseconds(),
		"subjects", []string{"cv.object.>", "cv.result.>"},
		"publish", "cv.alert",
	)

	ticker := time.NewTicker(cfg.PruneEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown complete")
			return
		case <-ticker.C:
			engine.Tick()
		default:
			// Sequential drains; Fetch blocks up to FetchTimeout each,
			// so this is not a tight spin.
			drain(objSub, cfg.FetchBatch, cfg.FetchTimeout, func(msg *nats.Msg) {
				if strings.HasPrefix(msg.Subject, "cv.object.") {
					engine.OnObject(msg.Data)
				}
				_ = msg.Ack()
			})
			drain(resSub, cfg.FetchBatch, cfg.FetchTimeout, func(msg *nats.Msg) {
				if strings.HasPrefix(msg.Subject, "cv.result.") {
					engine.OnResult(msg.Data)
				}
				_ = msg.Ack()
			})
		}
	}
}

func drain(sub *natsjs.Sub, batch int, timeout time.Duration, fn func(*nats.Msg)) {
	msgs, err := sub.Fetch(batch, timeout)
	if err != nil {
		slog.Warn("fetch", "subject", sub.Subject, "err", err)
		return
	}
	for _, m := range msgs {
		fn(m)
	}
}
