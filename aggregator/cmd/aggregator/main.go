// Aggregator: correlates ObjectEnvelopes + CapabilityResults by
// (frame_id, track_id), applies watchlist, emits final Alert to cv.alert.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/agentjetson/core/aggregator/internal/config"
	"github.com/agentjetson/core/aggregator/internal/correlate"
	"github.com/agentjetson/core/aggregator/internal/natsjs"
	"github.com/agentjetson/core/pkg/otel"
	"github.com/nats-io/nats.go"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := otel.Init(ctx, "aggregator")
	if err != nil {
		slog.Error("otel", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()

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

	publish := func(pctx context.Context, subject string, data []byte) error {
		msg := &nats.Msg{Subject: subject, Data: data, Header: make(nats.Header)}
		pctx, span := otel.StartProducerSpan(pctx, "aggregator", "nats.publish "+subject, msg)
		defer span.End()
		_, err := js.PublishMsg(msg, nats.Context(pctx))
		if err != nil {
			otel.RecordError(span, err)
		}
		return err
	}

	engine := correlate.New(
		correlate.ParseWatchlist(cfg.WatchlistRaw),
		cfg.EmitTimeout,
		cfg.PruneTimeout,
		publish,
	)

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
			drain(objSub, cfg.FetchBatch, cfg.FetchTimeout, func(msg *nats.Msg) {
				mctx, span := otel.StartConsumerSpan(context.Background(), "aggregator", "process.object", msg)
				defer span.End()
				if strings.HasPrefix(msg.Subject, "cv.object.") {
					engine.OnObject(mctx, msg.Data)
				}
				_ = msg.Ack()
			})
			drain(resSub, cfg.FetchBatch, cfg.FetchTimeout, func(msg *nats.Msg) {
				mctx, span := otel.StartConsumerSpan(context.Background(), "aggregator", "process.result", msg)
				defer span.End()
				if strings.HasPrefix(msg.Subject, "cv.result.") {
					engine.OnResult(mctx, msg.Data)
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
