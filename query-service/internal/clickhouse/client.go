package clickhouse

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/agentjetson/voice-query-service/internal/config"
	"github.com/agentjetson/voice-query-service/internal/models"
)

// Client is the ClickHouse-backed query backend.
// Default build is dependency-free: New() returns an error so the server
// falls back to DEMO_MODE. Enable the real driver with:
//
//	go get github.com/ClickHouse/clickhouse-go/v2@v2.30.0
//	go build -tags clickhouse ./cmd/server
//
// (see client_clickhouse.go)

type Client struct {
	cfg  *config.Config
	impl queryBackend
}

type queryBackend interface {
	Ping(ctx context.Context) error
	Close() error
	ApplySchema(ctx context.Context) error
	QueryRecentPlates(ctx context.Context, args models.QueryRecentPlatesArgs) ([]models.PlateHit, error)
	QueryObjects(ctx context.Context, args models.QueryObjectsArgs) ([]models.ObjectHit, error)
	QueryScenes(ctx context.Context, args models.QueryScenesArgs) ([]models.SceneHit, error)
	QueryDetections(ctx context.Context, args models.QueryAlertsArgs) ([]models.DetectionHit, error)
	QueryTranscripts(ctx context.Context, args models.QueryTranscriptsArgs) ([]models.TranscriptHit, error)
}

func New(cfg *config.Config) (*Client, error) {
	if cfg.DemoMode {
		return &Client{cfg: cfg}, nil
	}
	impl, err := newRealBackend(cfg)
	if err != nil {
		return nil, err
	}
	c := &Client{cfg: cfg, impl: impl}
	if cfg.ApplySchema {
		ctx := context.Background()
		if err := impl.ApplySchema(ctx); err != nil {
			slog.Warn("apply schema failed (tables may already exist)", "err", err)
		}
	}
	return c, nil
}

func (c *Client) Close() error {
	if c.impl == nil {
		return nil
	}
	return c.impl.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	if c.impl == nil {
		return nil
	}
	return c.impl.Ping(ctx)
}

func (c *Client) QueryRecentPlates(ctx context.Context, args models.QueryRecentPlatesArgs) ([]models.PlateHit, error) {
	if c.impl == nil {
		return nil, fmt.Errorf("clickhouse not connected (demo mode or missing driver)")
	}
	return c.impl.QueryRecentPlates(ctx, args)
}

func (c *Client) QueryObjects(ctx context.Context, args models.QueryObjectsArgs) ([]models.ObjectHit, error) {
	if c.impl == nil {
		return nil, fmt.Errorf("clickhouse not connected")
	}
	return c.impl.QueryObjects(ctx, args)
}

func (c *Client) QueryScenes(ctx context.Context, args models.QueryScenesArgs) ([]models.SceneHit, error) {
	if c.impl == nil {
		return nil, fmt.Errorf("clickhouse not connected")
	}
	return c.impl.QueryScenes(ctx, args)
}

func (c *Client) QueryDetections(ctx context.Context, args models.QueryAlertsArgs) ([]models.DetectionHit, error) {
	if c.impl == nil {
		return nil, fmt.Errorf("clickhouse not connected")
	}
	return c.impl.QueryDetections(ctx, args)
}

func (c *Client) QueryTranscripts(ctx context.Context, args models.QueryTranscriptsArgs) ([]models.TranscriptHit, error) {
	if c.impl == nil {
		return nil, fmt.Errorf("clickhouse not connected")
	}
	return c.impl.QueryTranscripts(ctx, args)
}
