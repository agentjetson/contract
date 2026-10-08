package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/agentjetson/voice-query-service/internal/clickhouse"
	"github.com/agentjetson/voice-query-service/internal/config"
	"github.com/agentjetson/voice-query-service/internal/models"
)

// Server holds dependencies for tool handlers.
type Server struct {
	cfg   *config.Config
	ch    *clickhouse.Client
	demo  *clickhouse.DemoStore
}

func New(cfg *config.Config, ch *clickhouse.Client) *Server {
	s := &Server{cfg: cfg, ch: ch}
	if cfg.DemoMode {
		s.demo = clickhouse.NewDemoStore()
	}
	return s
}

// QueryRecentPlates implements the primary ALPR tool.
func (s *Server) QueryRecentPlates(ctx context.Context, args models.QueryRecentPlatesArgs) ([]models.PlateHit, error) {
	if args.TimeWindowSec <= 0 {
		args.TimeWindowSec = s.cfg.DefaultTimeWindowSec
	}
	if args.Limit <= 0 {
		args.Limit = s.cfg.DefaultLimit
	}

	if s.cfg.DemoMode {
		return s.demo.QueryRecentPlates(args), nil
	}
	return s.ch.QueryRecentPlates(ctx, args)
}

func (s *Server) QueryObjects(ctx context.Context, args models.QueryObjectsArgs) ([]models.ObjectHit, error) {
	if args.TimeWindowSec <= 0 {
		args.TimeWindowSec = s.cfg.DefaultTimeWindowSec
	}
	if args.Limit <= 0 {
		args.Limit = s.cfg.DefaultLimit
	}
	if s.cfg.DemoMode {
		return s.demo.QueryObjects(args), nil
	}
	return s.ch.QueryObjects(ctx, args)
}

func (s *Server) QueryScenes(ctx context.Context, args models.QueryScenesArgs) ([]models.SceneHit, error) {
	if args.TimeWindowSec <= 0 {
		args.TimeWindowSec = s.cfg.DefaultTimeWindowSec
	}
	if args.Limit <= 0 {
		args.Limit = s.cfg.DefaultLimit
	}
	if s.cfg.DemoMode {
		return s.demo.QueryScenes(args), nil
	}
	return s.ch.QueryScenes(ctx, args)
}

func (s *Server) QueryDetections(ctx context.Context, args models.QueryAlertsArgs) ([]models.DetectionHit, error) {
	if args.TimeWindowSec <= 0 {
		args.TimeWindowSec = s.cfg.DefaultTimeWindowSec
	}
	if args.Limit <= 0 {
		args.Limit = s.cfg.DefaultLimit
	}
	if s.cfg.DemoMode {
		return s.demo.QueryDetections(args), nil
	}
	return s.ch.QueryDetections(ctx, args)
}

func (s *Server) QueryTranscripts(ctx context.Context, args models.QueryTranscriptsArgs) ([]models.TranscriptHit, error) {
	if args.TimeWindowSec <= 0 {
		args.TimeWindowSec = s.cfg.DefaultTimeWindowSec
	}
	if args.Limit <= 0 {
		args.Limit = s.cfg.DefaultLimit
	}
	if s.cfg.DemoMode {
		return s.demo.QueryTranscripts(args), nil
	}
	return s.ch.QueryTranscripts(ctx, args)
}

// Health returns service status.
func (s *Server) Health(ctx context.Context) models.HealthResponse {
	h := models.HealthResponse{
		Status:   "ok",
		DemoMode: s.cfg.DemoMode,
		Version:  "0.1.0",
	}
	if s.cfg.DemoMode {
		h.ClickHouse = "demo"
		return h
	}
	if err := s.ch.Ping(ctx); err != nil {
		h.Status = "degraded"
		h.ClickHouse = "unreachable: " + err.Error()
		slog.Warn("clickhouse ping failed", "err", err)
	} else {
		h.ClickHouse = "ok"
	}
	return h
}

// ParseJSONArgs is a helper for HTTP / MCP handlers.
func ParseJSONArgs[T any](raw json.RawMessage) (T, error) {
	var v T
	if len(raw) == 0 {
		return v, nil
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("invalid args: %w", err)
	}
	return v, nil
}
