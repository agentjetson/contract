//go:build clickhouse

package clickhouse

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/agentjetson/voice-query-service/internal/config"
	"github.com/agentjetson/voice-query-service/internal/models"
)

type realBackend struct {
	conn driver.Conn
	cfg  *config.Config
}

func newRealBackend(cfg *config.Config) (queryBackend, error) {
	opts := &ch.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.ClickHouseHost, cfg.ClickHousePort)},
		Auth: ch.Auth{
			Database: cfg.ClickHouseDatabase,
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		},
		DialTimeout: 5 * time.Second,
		Settings: ch.Settings{
			"max_execution_time": 60,
		},
	}
	conn, err := ch.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}
	return &realBackend{conn: conn, cfg: cfg}, nil
}

func (b *realBackend) Close() error { return b.conn.Close() }
func (b *realBackend) Ping(ctx context.Context) error { return b.conn.Ping(ctx) }

func (b *realBackend) ApplySchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS cv_results (
			event_time DateTime64(3),
			capability LowCardinality(String),
			plate String,
			confidence Float64,
			track_id String,
			camera_id LowCardinality(String),
			vehicle_class LowCardinality(String),
			make_model String,
			color LowCardinality(String),
			scene_l1 LowCardinality(String),
			scene_l2 LowCardinality(String),
			raw_json String
		) ENGINE = MergeTree()
		ORDER BY (event_time, camera_id, track_id)`,
		`CREATE TABLE IF NOT EXISTS cv_objects (
			event_time DateTime64(3),
			class LowCardinality(String),
			track_id String,
			camera_id LowCardinality(String),
			confidence Float64,
			bbox Array(Float32),
			scene_l1 LowCardinality(String),
			scene_l2 LowCardinality(String)
		) ENGINE = MergeTree()
		ORDER BY (event_time, camera_id, track_id)`,
		`CREATE TABLE IF NOT EXISTS cv_scenes (
			event_time DateTime64(3),
			level1 LowCardinality(String),
			level2 LowCardinality(String),
			camera_id LowCardinality(String),
			confidence Float64,
			specialists Array(String)
		) ENGINE = MergeTree()
		ORDER BY (event_time, camera_id)`,
		`CREATE TABLE IF NOT EXISTS cv_detections (
			event_time DateTime64(3),
			id String,
			class LowCardinality(String),
			camera_id LowCardinality(String),
			track_id String,
			confidence Float64,
			metadata String
		) ENGINE = MergeTree()
		ORDER BY (event_time, camera_id)`,
		`CREATE TABLE IF NOT EXISTS audio_transcripts (
			event_time DateTime64(3),
			text String,
			speaker LowCardinality(String),
			device_id LowCardinality(String),
			confidence Float64
		) ENGINE = MergeTree()
		ORDER BY (event_time, device_id)`,
	}
	for _, s := range stmts {
		if err := b.conn.Exec(ctx, s); err != nil {
			return fmt.Errorf("schema: %w", err)
		}
	}
	slog.Info("ClickHouse schema applied")
	return nil
}

func (b *realBackend) QueryRecentPlates(ctx context.Context, args models.QueryRecentPlatesArgs) ([]models.PlateHit, error) {
	window := args.TimeWindowSec
	if window <= 0 {
		window = b.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = b.cfg.DefaultLimit
	}
	vehicleClass, makeModel, color, sceneL1 := "", "", "", ""
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

	q := `
		SELECT plate, confidence, event_time, track_id, camera_id,
		       vehicle_class, make_model, color, scene_l1, scene_l2
		FROM cv_results
		WHERE capability = 'alpr'
		  AND event_time >= now() - INTERVAL ? SECOND
		  AND (? = '' OR vehicle_class = ?)
		  AND (? = '' OR lower(make_model) LIKE lower(concat('%', ?, '%')))
		  AND (? = '' OR lower(color) = lower(?))
		  AND (? = '' OR scene_l1 = ?)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := b.conn.Query(ctx, q,
		window,
		vehicleClass, vehicleClass,
		makeModel, makeModel,
		color, color,
		sceneL1, sceneL1,
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

func (b *realBackend) QueryObjects(ctx context.Context, args models.QueryObjectsArgs) ([]models.ObjectHit, error) {
	window := args.TimeWindowSec
	if window <= 0 {
		window = b.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = b.cfg.DefaultLimit
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
		FROM cv_objects
		WHERE event_time >= now() - INTERVAL ? SECOND
		  AND (? = '' OR class = ?)
		  AND (? = '' OR track_id = ?)
		  AND (? = '' OR camera_id = ?)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := b.conn.Query(ctx, q, window, class, class, trackID, trackID, cameraID, cameraID, limit)
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

func (b *realBackend) QueryScenes(ctx context.Context, args models.QueryScenesArgs) ([]models.SceneHit, error) {
	window := args.TimeWindowSec
	if window <= 0 {
		window = b.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = b.cfg.DefaultLimit
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
		FROM cv_scenes
		WHERE event_time >= now() - INTERVAL ? SECOND
		  AND (? = '' OR level1 = ?)
		  AND (? = '' OR camera_id = ?)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := b.conn.Query(ctx, q, window, l1, l1, cam, cam, limit)
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

func (b *realBackend) QueryDetections(ctx context.Context, args models.QueryAlertsArgs) ([]models.DetectionHit, error) {
	window := args.TimeWindowSec
	if window <= 0 {
		window = b.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = b.cfg.DefaultLimit
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
		FROM cv_detections
		WHERE event_time >= now() - INTERVAL ? SECOND
		  AND (? = '' OR class = ?)
		  AND (? = '' OR camera_id = ?)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := b.conn.Query(ctx, q, window, typ, typ, cam, cam, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.DetectionHit
	for rows.Next() {
		var h models.DetectionHit
		var metaStr string
		if err := rows.Scan(&h.ID, &h.Class, &h.CameraID, &h.TrackID, &h.Confidence, &h.Timestamp, &metaStr); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (b *realBackend) QueryTranscripts(ctx context.Context, args models.QueryTranscriptsArgs) ([]models.TranscriptHit, error) {
	window := args.TimeWindowSec
	if window <= 0 {
		window = b.cfg.DefaultTimeWindowSec
	}
	limit := args.Limit
	if limit <= 0 {
		limit = b.cfg.DefaultLimit
	}
	contains := ""
	if args.Contains != nil {
		contains = *args.Contains
	}
	q := `
		SELECT text, speaker, device_id, confidence, event_time
		FROM audio_transcripts
		WHERE event_time >= now() - INTERVAL ? SECOND
		  AND (? = '' OR positionCaseInsensitive(text, ?) > 0)
		ORDER BY event_time DESC
		LIMIT ?
	`
	rows, err := b.conn.Query(ctx, q, window, contains, contains, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.TranscriptHit
	for rows.Next() {
		var h models.TranscriptHit
		if err := rows.Scan(&h.Text, &h.Speaker, &h.CameraID, &h.Confidence, &h.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
