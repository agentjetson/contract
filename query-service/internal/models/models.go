package models

import "time"

// Shared request/response types used by tools, HTTP, and MCP handlers.

type QueryRecentPlatesArgs struct {
	MakeModel     *string `json:"make_model,omitempty"`
	VehicleClass  *string `json:"vehicle_class,omitempty"`
	Color         *string `json:"color,omitempty"`
	SceneL1       *string `json:"scene_l1,omitempty"`
	CameraID      *string `json:"camera_id,omitempty"`
	TimeWindowSec int     `json:"time_window_sec,omitempty"`
	Limit         int     `json:"limit,omitempty"`
}

type PlateHit struct {
	Plate        string    `json:"plate"`
	Confidence   float64   `json:"confidence"`
	Timestamp    time.Time `json:"observed_at"`
	TrackID      string    `json:"track_id"`
	CameraID     string    `json:"camera_id"`
	VehicleClass string    `json:"vehicle_class"`
	MakeModel    string    `json:"make_model"`
	Color        string    `json:"color"`
	SceneL1      string    `json:"scene_l1"`
	SceneL2      string    `json:"scene_l2"`
}

type QueryObjectsArgs struct {
	Class         *string `json:"class,omitempty"`
	TrackID       *string `json:"track_id,omitempty"`
	CameraID      *string `json:"camera_id,omitempty"`
	TimeWindowSec int     `json:"time_window_sec,omitempty"`
	Limit         int     `json:"limit,omitempty"`
}

type ObjectHit struct {
	TrackID    string    `json:"track_id"`
	Class      string    `json:"class"`
	CameraID   string    `json:"camera_id"`
	Confidence float64   `json:"confidence"`
	BBox       []float32 `json:"bbox,omitempty"`
	SceneL1    string    `json:"scene_l1"`
	SceneL2    string    `json:"scene_l2"`
	Timestamp  time.Time `json:"observed_at"`
}

type QueryScenesArgs struct {
	Level1        *string `json:"level1,omitempty"`
	CameraID      *string `json:"camera_id,omitempty"`
	TimeWindowSec int     `json:"time_window_sec,omitempty"`
	Limit         int     `json:"limit,omitempty"`
}

type SceneHit struct {
	CameraID     string    `json:"camera_id"`
	Level1       string    `json:"level1"`
	Level2       string    `json:"level2"`
	Confidence   float64   `json:"confidence"`
	Specialists  []string  `json:"specialists,omitempty"`
	Timestamp    time.Time `json:"observed_at"`
}

type QueryAlertsArgs struct {
	Type          *string `json:"type,omitempty"`
	CameraID      *string `json:"camera_id,omitempty"`
	TimeWindowSec int     `json:"time_window_sec,omitempty"`
	Limit         int     `json:"limit,omitempty"`
}

type DetectionHit struct {
	ID         string    `json:"id"`
	Class      string    `json:"class"`
	CameraID   string    `json:"camera_id"`
	TrackID    string    `json:"track_id"`
	Confidence float64   `json:"confidence"`
	Timestamp  time.Time `json:"observed_at"`
	Metadata   string    `json:"metadata,omitempty"`
}

type QueryTranscriptsArgs struct {
	Contains      *string `json:"contains,omitempty"`
	TimeWindowSec int     `json:"time_window_sec,omitempty"`
	Limit         int     `json:"limit,omitempty"`
}

type TranscriptHit struct {
	Text       string    `json:"text"`
	Speaker    string    `json:"speaker,omitempty"`
	DeviceID   string    `json:"device_id,omitempty"`
	Confidence float64   `json:"confidence"`
	Timestamp  time.Time `json:"observed_at"`
}

type HealthResponse struct {
	Status     string `json:"status"`
	DemoMode   bool   `json:"demo_mode"`
	ClickHouse string `json:"clickhouse"`
	Version    string `json:"version"`
}
