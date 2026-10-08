package clickhouse

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/agentjetson/core/pkg/persistence"
	"github.com/agentjetson/voice-query-service/internal/config"
	"github.com/agentjetson/voice-query-service/internal/models"
)

// Client is the ClickHouse-backed query backend.
// Always uses pkg/persistence.Open — never issues CREATE TABLE.
// SELECTs go against query_* views (seed/sql/003_views.sql).

type Client struct {
	cfg *config.Config
	ch  *persistence.Client
}

func New(cfg *config.Config) (*Client, error) {
	if cfg.DemoMode {
		return &Client{cfg: cfg}, nil
	}
	ch, err := persistence.Open(persistence.Config{
		Host:     cfg.ClickHouseHost,
		Port:     cfg.ClickHousePort,
		User:     cfg.ClickHouseUser,
		Password: cfg.ClickHousePassword,
		Database: cfg.ClickHouseDatabase,
	})
	if err != nil {
		return nil, err
	}
	if cfg.ApplySchema {
		slog.Warn("APPLY_SCHEMA is ignored — schema is owned by contract/seed/sql (make schema)")
	}
	return &Client{cfg: cfg, ch: ch}, nil
}

func (c *Client) Close() error {
	if c.ch == nil {
		return nil
	}
	return c.ch.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	if c.ch == nil {
		return nil
	}
	return c.ch.Ping(ctx)
}

func (c *Client) QueryRecentPlates(ctx context.Context, args models.QueryRecentPlatesArgs) ([]models.PlateHit, error) {
	if c.ch == nil {
		return nil, fmt.Errorf("clickhouse not connected (demo mode)")
	}
	window := args.TimeWindowSec
	if window <= 0 {
		window = c.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = c.cfg.DefaultLimit
	}
	vehicleClass, makeModel, color, sceneL1, cameraID := "", "", "", "", ""
	if args.VehicleClass != nil {
		vehicleClass = *args.VehicleClass
	}
	if args.MakeModel != nil {
		makeModel = *args.MakeModel
	}
	if args.Color != nil {
		color = *args.Color
	}
	if args.SceneL1 != nil {
		sceneL1 = *args.SceneL1
	}
	if args.CameraID != nil {
		cameraID = *args.CameraID
	}

	q := `
		SELECT plate, confidence, event_time, track_id, camera_id,
		       vehicle_class, make_model, color, scene_l1, scene_l2
		FROM query_cv_results
		WHERE capability = 'alpr'
		  AND event_time >= now64(3) - INTERVAL ? SECOND
		  AND (? = '' OR vehicle_class = ?)
		  AND (? = '' OR lower(make_model) LIKE lower(concat('%', ?, '%')))
		  AND (? = '' OR lower(color) = lower(?))
		  AND (? = '' OR scene_l1 = ?)
		  AND (? = '' OR camera_id = ?)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := c.ch.Conn().Query(ctx, q,
		window,
		vehicleClass, vehicleClass,
		makeModel, makeModel,
		color, color,
		sceneL1, sceneL1,
		cameraID, cameraID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query plates: %w", err)
	}
	defer rows.Close()

	var out []models.PlateHit
	for rows.Next() {
		var h models.PlateHit
		if err := rows.Scan(
			&h.Plate, &h.Confidence, &h.Timestamp, &h.TrackID, &h.CameraID,
			&h.VehicleClass, &h.MakeModel, &h.Color, &h.SceneL1, &h.SceneL2,
		); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (c *Client) QueryObjects(ctx context.Context, args models.QueryObjectsArgs) ([]models.ObjectHit, error) {
	if c.ch == nil {
		return nil, fmt.Errorf("clickhouse not connected")
	}
	window := args.TimeWindowSec
	if window <= 0 {
		window = c.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = c.cfg.DefaultLimit
	}
	class, trackID, cameraID := "", "", ""
	if args.Class != nil {
		class = *args.Class
	}
	if args.TrackID != nil {
		trackID = *args.TrackID
	}
	if args.CameraID != nil {
		cameraID = *args.CameraID
	}
	q := `
		SELECT class, track_id, camera_id, confidence, event_time, bbox, scene_l1, scene_l2
		FROM query_cv_objects
		WHERE event_time >= now64(3) - INTERVAL ? SECOND
		  AND (? = '' OR class = ?)
		  AND (? = '' OR track_id = ?)
		  AND (? = '' OR camera_id = ?)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := c.ch.Conn().Query(ctx, q, window, class, class, trackID, trackID, cameraID, cameraID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ObjectHit
	for rows.Next() {
		var h models.ObjectHit
		if err := rows.Scan(&h.Class, &h.TrackID, &h.CameraID, &h.Confidence, &h.Timestamp, &h.BBox, &h.SceneL1, &h.SceneL2); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (c *Client) QueryScenes(ctx context.Context, args models.QueryScenesArgs) ([]models.SceneHit, error) {
	if c.ch == nil {
		return nil, fmt.Errorf("clickhouse not connected")
	}
	window := args.TimeWindowSec
	if window <= 0 {
		window = c.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = c.cfg.DefaultLimit
	}
	l1, cam := "", ""
	if args.Level1 != nil {
		l1 = *args.Level1
	}
	if args.CameraID != nil {
		cam = *args.CameraID
	}
	q := `
		SELECT level1, level2, camera_id, confidence, event_time, specialists
		FROM query_cv_scenes
		WHERE event_time >= now64(3) - INTERVAL ? SECOND
		  AND (? = '' OR level1 = ?)
		  AND (? = '' OR camera_id = ?)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := c.ch.Conn().Query(ctx, q, window, l1, l1, cam, cam, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.SceneHit
	for rows.Next() {
		var h models.SceneHit
		if err := rows.Scan(&h.Level1, &h.Level2, &h.CameraID, &h.Confidence, &h.Timestamp, &h.Specialists); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (c *Client) QueryDetections(ctx context.Context, args models.QueryAlertsArgs) ([]models.DetectionHit, error) {
	if c.ch == nil {
		return nil, fmt.Errorf("clickhouse not connected")
	}
	window := args.TimeWindowSec
	if window <= 0 {
		window = c.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = c.cfg.DefaultLimit
	}
	typ, cam := "", ""
	if args.Type != nil {
		typ = *args.Type
	}
	if args.CameraID != nil {
		cam = *args.CameraID
	}
	q := `
		SELECT id, class, camera_id, track_id, confidence, event_time, metadata
		FROM query_cv_detections
		WHERE event_time >= now64(3) - INTERVAL ? SECOND
		  AND (? = '' OR class = ?)
		  AND (? = '' OR camera_id = ?)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := c.ch.Conn().Query(ctx, q, window, typ, typ, cam, cam, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.DetectionHit
	for rows.Next() {
		var h models.DetectionHit
		if err := rows.Scan(&h.ID, &h.Class, &h.CameraID, &h.TrackID, &h.Confidence, &h.Timestamp, &h.Metadata); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (c *Client) QueryTranscripts(ctx context.Context, args models.QueryTranscriptsArgs) ([]models.TranscriptHit, error) {
	if c.ch == nil {
		return nil, fmt.Errorf("clickhouse not connected")
	}
	window := args.TimeWindowSec
	if window <= 0 {
		window = c.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = c.cfg.DefaultLimit
	}
	contains := ""
	if args.Contains != nil {
		contains = *args.Contains
	}
	q := `
		SELECT text, speaker, device_id, confidence, event_time
		FROM query_audio_transcripts
		WHERE event_time >= now64(3) - INTERVAL ? SECOND
		  AND (? = '' OR positionCaseInsensitive(text, ?) > 0)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := c.ch.Conn().Query(ctx, q, window, contains, contains, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.TranscriptHit
	for rows.Next() {
		var h models.TranscriptHit
		if err := rows.Scan(&h.Text, &h.Speaker, &h.DeviceID, &h.Confidence, &h.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
