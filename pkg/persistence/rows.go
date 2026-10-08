package persistence

import "time"

// Row types mirror seed/sql/002_tables.sql physical columns.
// Time is always UTC truncated to milliseconds (DateTime64(3)).
// Labels / attributes are ClickHouse Map columns.
// JPEG / crop bytes are never stored.

// ObjectRow ← detection.v1.ObjectEnvelope → cv_objects
type ObjectRow struct {
	FrameID          int64
	TS               time.Time
	Source           string
	ClassName        string
	ClassID          int32
	Confidence       float32
	X1, Y1, X2, Y2   float32
	TrackID          int32
	FrameWidth       int32
	FrameHeight      int32
	CaptureLatencyMs float64
	SceneL1          string
	SceneL2          string
	NatsSeq          uint64
	Labels           map[string]string
}

// ResultRow ← detection.v1.CapabilityResult → cv_results
type ResultRow struct {
	FrameID                int64
	TS                     time.Time
	Source                 string
	Capability             string
	TrackID                int32
	ClassName              string
	Confidence             float32
	X1, Y1, X2, Y2         float32
	OCRText                string
	OCRConfidence          float32
	PlateX1, PlateY1       float32
	PlateX2, PlateY2       float32
	ProcessingMs           float64
	VehicleClass           string
	MakeModel              string
	Color                  string
	SceneL1                string
	SceneL2                string
	NatsSeq                uint64
	Attributes             map[string]string
	Labels                 map[string]string
}

// SceneRow ← scene.v1.SceneResult → cv_scenes
type SceneRow struct {
	FrameID           int64
	TS                time.Time
	Source            string
	Level1            string
	Level2            string
	Level1Confidence  float32
	Level2Confidence  float32
	Specialists       []string
	Backend           string
	TemporalRequested uint8
	Notes             string
	NatsSeq           uint64
}

// DetectionRow ← detection.v1.Alert (1 per Detection) → cv_detections
type DetectionRow struct {
	FrameID       int64
	TS            time.Time
	Source        string
	WatchlistHit  uint8
	MatchedLabel  string
	E2ELatencyMs  float64
	ClassID       int32
	ClassName     string
	Confidence    float32
	X1, Y1, X2, Y2 float32
	TrackID       int32
	NatsSeq       uint64
	Labels        map[string]string
}

// TranscriptRow ← audio.v1.Transcript → audio_transcripts
type TranscriptRow struct {
	TS, AudioStart, AudioEnd time.Time
	Source                   string
	Text                     string
	IsFinal                  uint8
	Confidence               float32
	Language                 string
	SpeakerID                string
	E2ELatencyMs             float64
	NatsSeq                  uint64
	Labels                   map[string]string
}

// ObjectMetaRow ← object-storage Put* → object_meta
type ObjectMetaRow struct {
	ObjectID       string
	Kind           string
	Source         string
	EventTS        time.Time
	FrameID        int64
	TrackID        int32
	ContentType    string
	SizeBytes      int64
	ChecksumSHA256 string
	StorageKey     string
	Bucket         string
	Labels         map[string]string
	Deleted        uint8
}
