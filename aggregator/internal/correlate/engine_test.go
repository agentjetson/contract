package correlate

import (
	"context"
	"sync"
	"testing"
	"time"

	detectionv1 "github.com/agentjetson/core/gen/go/detection/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestParseWatchlistDefaults(t *testing.T) {
	w := ParseWatchlist("")
	if len(w) < 3 {
		t.Fatalf("expected default watchlist, got %v", w)
	}
}

func TestParseWatchlistCustom(t *testing.T) {
	w := ParseWatchlist("person:0.7,ABC123:0.9")
	if len(w) != 2 {
		t.Fatalf("got %d entries", len(w))
	}
	if w[0].Label != "person" || w[0].MinConf != 0.7 {
		t.Fatalf("person entry: %+v", w[0])
	}
	if w[1].Label != "ABC123" || w[1].MinConf != 0.9 {
		t.Fatalf("plate entry: %+v", w[1])
	}
}

func TestCorrelateObjectThenALPR(t *testing.T) {
	var (
		mu        sync.Mutex
		published [][]byte
	)
	publish := func(_ context.Context, _ string, data []byte) error {
		mu.Lock()
		defer mu.Unlock()
		published = append(published, append([]byte(nil), data...))
		return nil
	}
	e := New(DefaultWatchlist(), 800*time.Millisecond, 5*time.Second, publish)

	obj := &detectionv1.ObjectEnvelope{
		FrameId:    42,
		Timestamp:  timestamppb.Now(),
		Source:     "cam-01",
		ClassName:  "car",
		ClassId:    2,
		Confidence: 0.9,
		TrackId:    7,
		Box:        &detectionv1.BoundingBox{X1: 1, Y1: 2, X2: 3, Y2: 4},
	}
	objBytes, _ := proto.Marshal(obj)
	e.OnObject(context.Background(), objBytes)

	mu.Lock()
	n := len(published)
	mu.Unlock()
	if n != 0 {
		t.Fatalf("should not emit before ALPR or timeout, got %d", n)
	}

	res := &detectionv1.CapabilityResult{
		FrameId:       42,
		Timestamp:     timestamppb.Now(),
		Source:        "cam-01",
		Capability:    "alpr",
		TrackId:       7,
		ClassName:     "car",
		OcrText:       "7ABC123",
		OcrConfidence: 0.95,
		PlateBox:      &detectionv1.BoundingBox{X1: 10, Y1: 20, X2: 30, Y2: 40},
	}
	resBytes, _ := proto.Marshal(res)
	e.OnResult(context.Background(), resBytes)

	mu.Lock()
	defer mu.Unlock()
	if len(published) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(published))
	}
	var alert detectionv1.Alert
	if err := proto.Unmarshal(published[0], &alert); err != nil {
		t.Fatal(err)
	}
	if alert.FrameId != 42 || !alert.WatchlistHit {
		t.Fatalf("alert: frame=%d hit=%v match=%q dets=%d",
			alert.FrameId, alert.WatchlistHit, alert.MatchedLabel, len(alert.Detections))
	}
	if len(alert.Detections) < 1 || alert.Detections[0].OcrText != "7ABC123" {
		t.Fatalf("detections: %+v", alert.Detections)
	}
}

func TestEmitOnTimeoutWithoutALPR(t *testing.T) {
	var published int
	publish := func(_ context.Context, _ string, _ []byte) error {
		published++
		return nil
	}
	e := New(DefaultWatchlist(), 1*time.Millisecond, 100*time.Millisecond, publish)
	obj := &detectionv1.ObjectEnvelope{
		FrameId:    1,
		Source:     "cam-01",
		ClassName:  "person",
		Confidence: 0.8,
		TrackId:    3,
	}
	objBytes, _ := proto.Marshal(obj)
	e.OnObject(context.Background(), objBytes)
	time.Sleep(5 * time.Millisecond)
	e.Tick()
	if published != 1 {
		t.Fatalf("expected timeout emit, got %d", published)
	}
}

func TestIgnoreNonALPRResult(t *testing.T) {
	var published int
	e := New(DefaultWatchlist(), time.Second, time.Second, func(_ context.Context, _ string, _ []byte) error {
		published++
		return nil
	})
	res := &detectionv1.CapabilityResult{
		FrameId:    1,
		Capability: "vehicle_attr",
		TrackId:    1,
	}
	b, _ := proto.Marshal(res)
	e.OnResult(context.Background(), b)
	if published != 0 {
		t.Fatalf("non-alpr should not emit, got %d", published)
	}
}
