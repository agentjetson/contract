package natsjs

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// Sub is a durable pull consumer on a JetStream subject filter.
type Sub struct {
	Stream  string
	Durable string
	Subject string
	sub     *nats.Subscription
}

func Connect(url, name string) (*nats.Conn, nats.JetStreamContext, error) {
	nc, err := nats.Connect(url,
		nats.Name(name),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("jetstream: %w", err)
	}
	return nc, js, nil
}

func WaitStream(js nats.JetStreamContext, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := js.StreamInfo(name); err == nil {
			slog.Info("stream ready", "stream", name)
			return nil
		}
		slog.Info("waiting for stream", "stream", name)
		time.Sleep(time.Second)
	}
	return fmt.Errorf("stream %s not ready within %s", name, timeout)
}

// EnsureSourceSubjects updates CV_EVENTS (or given stream) to include cv.source.>
// so dynamic registration messages are captured. Best-effort; logs on failure.
func EnsureSourceSubjects(js nats.JetStreamContext, stream string) {
	info, err := js.StreamInfo(stream)
	if err != nil {
		slog.Debug("EnsureSourceSubjects: stream missing", "stream", stream, "err", err)
		return
	}
	has := false
	for _, s := range info.Config.Subjects {
		if s == "cv.source.>" || s == "cv.source.*" {
			has = true
			break
		}
	}
	if has {
		return
	}
	cfg := info.Config
	cfg.Subjects = append(cfg.Subjects, "cv.source.>")
	if _, err := js.UpdateStream(&cfg); err != nil {
		slog.Warn("EnsureSourceSubjects: UpdateStream failed — ensure CV_EVENTS includes cv.source.>",
			"stream", stream, "err", err)
		return
	}
	slog.Info("stream subjects updated", "stream", stream, "added", "cv.source.>")
}

func EnsurePull(js nats.JetStreamContext, stream, durable, subject string) (*Sub, error) {
	_, err := js.AddConsumer(stream, &nats.ConsumerConfig{
		Durable:       durable,
		AckPolicy:     nats.AckExplicitPolicy,
		FilterSubject: subject,
		DeliverPolicy: nats.DeliverNewPolicy,
	})
	if err != nil {
		slog.Debug("AddConsumer", "durable", durable, "err", err)
	}
	sub, err := js.PullSubscribe(subject, durable, nats.Bind(stream, durable))
	if err != nil {
		sub, err = js.PullSubscribe(subject, durable)
		if err != nil {
			return nil, fmt.Errorf("PullSubscribe %s: %w", subject, err)
		}
	}
	return &Sub{Stream: stream, Durable: durable, Subject: subject, sub: sub}, nil
}

func (s *Sub) Fetch(batch int, timeout time.Duration) ([]*nats.Msg, error) {
	msgs, err := s.sub.Fetch(batch, nats.MaxWait(timeout))
	if err == nats.ErrTimeout {
		return nil, nil
	}
	return msgs, err
}

func (s *Sub) Close() {
	if s != nil && s.sub != nil {
		_ = s.sub.Unsubscribe()
	}
}
