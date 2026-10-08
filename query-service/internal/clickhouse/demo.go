package clickhouse

import (
	"strings"
	"time"

	"github.com/agentjetson/voice-query-service/internal/models"
)

// DemoStore provides in-memory sample data for DEMO_MODE=true.
// Matches the documented demo: plate 7ABC123, Mercedes C-Class, roadway/driving, track 77.
type DemoStore struct{}

func NewDemoStore() *DemoStore {
	return &DemoStore{}
}

func (d *DemoStore) QueryRecentPlates(args models.QueryRecentPlatesArgs) []models.PlateHit {
	now := time.Now().UTC()
	samples := []models.PlateHit{
		{
			Plate:        "7ABC123",
			Confidence:   0.94,
			Timestamp:    now.Add(-8 * time.Second),
			TrackID:      "77",
			CameraID:     "cam-01",
			VehicleClass: "car",
			MakeModel:    "Mercedes C-Class",
			Color:        "silver",
			SceneL1:      "roadway",
			SceneL2:      "driving",
		},
		{
			Plate:        "9XYZ456",
			Confidence:   0.88,
			Timestamp:    now.Add(-22 * time.Second),
			TrackID:      "81",
			CameraID:     "cam-01",
			VehicleClass: "truck",
			MakeModel:    "Ford F-150",
			Color:        "blue",
			SceneL1:      "roadway",
			SceneL2:      "driving",
		},
		{
			Plate:        "4DEF789",
			Confidence:   0.91,
			Timestamp:    now.Add(-45 * time.Second),
			TrackID:      "65",
			CameraID:     "cam-02",
			VehicleClass: "car",
			MakeModel:    "Toyota Camry",
			Color:        "white",
			SceneL1:      "roadway",
			SceneL2:      "stopped",
		},
	}

	window := args.TimeWindowSec
	if window <= 0 {
		window = 30
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}
	cutoff := now.Add(-time.Duration(window) * time.Second)

	var out []models.PlateHit
	for _, h := range samples {
		if h.Timestamp.Before(cutoff) {
			continue
		}
		if args.VehicleClass != nil && *args.VehicleClass != "" && !strings.EqualFold(h.VehicleClass, *args.VehicleClass) {
			continue
		}
		if args.MakeModel != nil && *args.MakeModel != "" && !strings.Contains(strings.ToLower(h.MakeModel), strings.ToLower(*args.MakeModel)) {
			continue
		}
		if args.Color != nil && *args.Color != "" && !strings.EqualFold(h.Color, *args.Color) {
			continue
		}
		if args.SceneL1 != nil && *args.SceneL1 != "" && !strings.EqualFold(h.SceneL1, *args.SceneL1) {
			continue
		}
		out = append(out, h)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (d *DemoStore) QueryObjects(args models.QueryObjectsArgs) []models.ObjectHit {
	now := time.Now().UTC()
	samples := []models.ObjectHit{
		{Class: "car", TrackID: "77", CameraID: "cam-01", Confidence: 0.96, Timestamp: now.Add(-8 * time.Second), SceneL1: "roadway", SceneL2: "driving"},
		{Class: "truck", TrackID: "81", CameraID: "cam-01", Confidence: 0.93, Timestamp: now.Add(-22 * time.Second), SceneL1: "roadway", SceneL2: "driving"},
		{Class: "person", TrackID: "12", CameraID: "cam-02", Confidence: 0.89, Timestamp: now.Add(-15 * time.Second), SceneL1: "sidewalk", SceneL2: "walking"},
	}
	window := args.TimeWindowSec
	if window <= 0 {
		window = 30
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}
	cutoff := now.Add(-time.Duration(window) * time.Second)
	var out []models.ObjectHit
	for _, h := range samples {
		if h.Timestamp.Before(cutoff) {
			continue
		}
		if args.Class != nil && *args.Class != "" && !strings.EqualFold(h.Class, *args.Class) {
			continue
		}
		if args.TrackID != nil && *args.TrackID != "" && h.TrackID != *args.TrackID {
			continue
		}
		if args.CameraID != nil && *args.CameraID != "" && h.CameraID != *args.CameraID {
			continue
		}
		out = append(out, h)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (d *DemoStore) QueryScenes(args models.QueryScenesArgs) []models.SceneHit {
	now := time.Now().UTC()
	samples := []models.SceneHit{
		{Level1: "roadway", Level2: "driving", CameraID: "cam-01", Confidence: 0.97, Timestamp: now.Add(-5 * time.Second), Specialists: []string{"alpr", "speed"}},
		{Level1: "sidewalk", Level2: "walking", CameraID: "cam-02", Confidence: 0.91, Timestamp: now.Add(-12 * time.Second), Specialists: []string{"person_attr"}},
	}
	window := args.TimeWindowSec
	if window <= 0 {
		window = 30
	}
	limit := args.Limit
	if limit <= 0 {
		limit = 5
	}
	cutoff := now.Add(-time.Duration(window) * time.Second)
	var out []models.SceneHit
	for _, h := range samples {
		if h.Timestamp.Before(cutoff) {
			continue
		}
		if args.Level1 != nil && *args.Level1 != "" && !strings.EqualFold(h.Level1, *args.Level1) {
			continue
		}
		if args.CameraID != nil && *args.CameraID != "" && h.CameraID != *args.CameraID {
			continue
		}
		out = append(out, h)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (d *DemoStore) QueryDetections(args models.QueryAlertsArgs) []models.DetectionHit {
	now := time.Now().UTC()
	return []models.DetectionHit{
		{ID: "det-001", Class: "watchlist_hit", CameraID: "cam-01", TrackID: "77", Confidence: 0.9, Timestamp: now.Add(-10 * time.Second)},
	}
}

func (d *DemoStore) QueryTranscripts(args models.QueryTranscriptsArgs) []models.TranscriptHit {
	now := time.Now().UTC()
	return []models.TranscriptHit{
		{Text: "Unit 12, vehicle matching description heading northbound.", Speaker: "dispatch", CameraID: "radio-1", Timestamp: now.Add(-20 * time.Second), Confidence: 0.95},
	}
}
