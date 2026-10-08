package natsjs

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// Connect opens a NATS connection and returns a JetStream context.
// name is used as the client name (appears in NATS monitoring).
func Connect(url, name string) (*nats.Conn, nats.JetStreamContext, error) {
	if name == "" {
		name = "agentjetson"
	}
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
