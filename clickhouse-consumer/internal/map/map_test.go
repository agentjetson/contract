package mapx

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestObjectToRowSceneFromLabels(t *testing.T) {
	ts := timestamppb.New(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	row := ObjectToRow(ObjectFields{
		FrameID: 10, Timestamp: ts, Source: "cam-01",
		ClassName: "car", ClassID: 2, Confidence: 0.9,
		X1: 1, Y1: 2, X2: 3, Y2: 4, TrackID: 7,
		Labels: map[string]string{"scene_l1": "roadway", "scene_l2": "driving"},
	}, 99)
	if row.SceneL1 != "roadway" || row.SceneL2 != "driving" {
		t.Fatalf("scene: %s/%s", row.SceneL1, row.SceneL2)
	}
	if row.NatsSeq != 99 || row.FrameID != 10 {
		t.Fatalf("row: %+v", row)
	}
}

func TestResultToRowAttrsAndOCRConfidence(t *testing.T) {
	row := ResultToRow(ResultFields{
		FrameID: 1, Source: "cam-01", Capability: "alpr", TrackID: 3,
		ClassName: "car", OCRText: "7ABC123", OCRConfidence: 0.94,
		Attributes: map[string]string{
			"make": "Mercedes", "model": "C-Class", "color": "silver",
		},
	}, 1)
	if row.Confidence != 0.94 {
		t.Fatalf("confidence from ocr: %v", row.Confidence)
	}
	if row.MakeModel != "Mercedes C-Class" {
		t.Fatalf("make_model: %q", row.MakeModel)
	}
	if row.Color != "silver" || row.VehicleClass != "car" {
		t.Fatalf("color/vc: %s/%s", row.Color, row.VehicleClass)
	}
}

func TestResultToRowMakeModelKey(t *testing.T) {
	row := ResultToRow(ResultFields{
		Capability: "alpr",
		Attributes: map[string]string{"make_model": "Toyota Camry", "vehicle_class": "car"},
	}, 0)
	if row.MakeModel != "Toyota Camry" || row.VehicleClass != "car" {
		t.Fatalf("%+v", row)
	}
}

func TestAlertToRowsOnePerDetection(t *testing.T) {
	rows := AlertToRows(AlertFields{
		FrameID: 5, Source: "cam-01", WatchlistHit: true, MatchedLabel: "car#7",
		Labels: map[string]string{"site": "hq"},
		Detections: []DetectionFields{
			{ClassID: 2, ClassName: "car", Confidence: 0.9, TrackID: 7},
			{ClassID: 1000, ClassName: "license_plate", Confidence: 0.95, TrackID: 7},
		},
	}, 42)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0].Labels["site"] != "hq" || rows[0].WatchlistHit != 1 {
		t.Fatalf("%+v", rows[0])
	}
	if rows[1].ClassName != "license_plate" {
		t.Fatalf("%+v", rows[1])
	}
}

func TestAlertToRowsEmptyDetections(t *testing.T) {
	rows := AlertToRows(AlertFields{FrameID: 1, Source: "cam-01"}, 1)
	if len(rows) != 1 || rows[0].ClassID != -1 {
		t.Fatalf("%+v", rows)
	}
}

func TestSceneToRow(t *testing.T) {
	row := SceneToRow(SceneFields{
		FrameID: 1, Source: "cam-01", Level1: "roadway", Level2: "driving",
		Level1Confidence: 0.9, Level2Confidence: 0.8,
		Specialists: []string{"alpr"}, Backend: "siglip2", TemporalRequested: true,
	}, 3)
	if row.TemporalRequested != 1 || len(row.Specialists) != 1 {
		t.Fatalf("%+v", row)
	}
}

func TestTranscriptToRowDefaults(t *testing.T) {
	row := TranscriptToRow(TranscriptFields{
		Source: "mic-01", Text: "hello", IsFinal: true, Confidence: 0.99,
	}, 7)
	if row.Language != "en" || row.IsFinal != 1 || row.NatsSeq != 7 {
		t.Fatalf("%+v", row)
	}
}
