// Package mapx converts contract protos into persistence row types.
package mapx

import (
	"time"

	"github.com/agentjetson/core/pkg/persistence"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ProtoTS(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return persistence.NowMS()
	}
	return persistence.TruncateMS(ts.AsTime())
}

func BoolU8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// AlertFields is the subset of detection.v1.Alert needed for mapping.
type AlertFields struct {
	FrameID      int64
	Timestamp    *timestamppb.Timestamp
	Source       string
	WatchlistHit bool
	MatchedLabel string
	E2ELatencyMs float64
	Labels       map[string]string
	Detections   []DetectionFields
}

type DetectionFields struct {
	ClassID        int32
	ClassName      string
	Confidence     float32
	X1, Y1, X2, Y2 float32
	TrackID        int32
}

func AlertToRows(a AlertFields, seq uint64) []persistence.DetectionRow {
	ts := ProtoTS(a.Timestamp)
	labels := a.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	if len(a.Detections) == 0 {
		return []persistence.DetectionRow{{
			FrameID: a.FrameID, TS: ts, Source: a.Source,
			WatchlistHit: BoolU8(a.WatchlistHit), MatchedLabel: a.MatchedLabel,
			E2ELatencyMs: a.E2ELatencyMs, ClassID: -1, NatsSeq: seq,
			Labels: labels,
		}}
	}
	out := make([]persistence.DetectionRow, 0, len(a.Detections))
	for _, d := range a.Detections {
		out = append(out, persistence.DetectionRow{
			FrameID: a.FrameID, TS: ts, Source: a.Source,
			WatchlistHit: BoolU8(a.WatchlistHit), MatchedLabel: a.MatchedLabel,
			E2ELatencyMs: a.E2ELatencyMs,
			ClassID:      d.ClassID, ClassName: d.ClassName, Confidence: d.Confidence,
			X1: d.X1, Y1: d.Y1, X2: d.X2, Y2: d.Y2,
			TrackID: d.TrackID, NatsSeq: seq,
			Labels: labels,
		})
	}
	return out
}

// TranscriptFields is the subset of audio.v1.Transcript needed for mapping.
type TranscriptFields struct {
	Timestamp    *timestamppb.Timestamp
	AudioStart   *timestamppb.Timestamp
	AudioEnd     *timestamppb.Timestamp
	Source       string
	Text         string
	IsFinal      bool
	Confidence   float32
	Language     string
	SpeakerID    string
	E2ELatencyMs float64
	Labels       map[string]string
}

func TranscriptToRow(t TranscriptFields, seq uint64) persistence.TranscriptRow {
	ts := ProtoTS(t.Timestamp)
	start := ts
	if t.AudioStart != nil {
		start = ProtoTS(t.AudioStart)
	}
	end := ts
	if t.AudioEnd != nil {
		end = ProtoTS(t.AudioEnd)
	}
	lang := t.Language
	if lang == "" {
		lang = "en"
	}
	labels := t.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	return persistence.TranscriptRow{
		TS: ts, AudioStart: start, AudioEnd: end,
		Source: t.Source, Text: t.Text, IsFinal: BoolU8(t.IsFinal),
		Confidence: t.Confidence, Language: lang, SpeakerID: t.SpeakerID,
		E2ELatencyMs: t.E2ELatencyMs, NatsSeq: seq,
		Labels: labels,
	}
}

// ObjectFields ← detection.v1.ObjectEnvelope
type ObjectFields struct {
	FrameID          int64
	Timestamp        *timestamppb.Timestamp
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
	Labels           map[string]string
}

func ObjectToRow(o ObjectFields, seq uint64) persistence.ObjectRow {
	labels := o.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	sceneL1 := o.SceneL1
	if sceneL1 == "" {
		sceneL1 = labels["scene_l1"]
	}
	sceneL2 := o.SceneL2
	if sceneL2 == "" {
		sceneL2 = labels["scene_l2"]
	}
	return persistence.ObjectRow{
		FrameID: o.FrameID, TS: ProtoTS(o.Timestamp), Source: o.Source,
		ClassName: o.ClassName, ClassID: o.ClassID, Confidence: o.Confidence,
		X1: o.X1, Y1: o.Y1, X2: o.X2, Y2: o.Y2, TrackID: o.TrackID,
		FrameWidth: o.FrameWidth, FrameHeight: o.FrameHeight,
		CaptureLatencyMs: o.CaptureLatencyMs, SceneL1: sceneL1, SceneL2: sceneL2,
		NatsSeq: seq, Labels: labels,
	}
}

// ResultFields ← detection.v1.CapabilityResult
type ResultFields struct {
	FrameID          int64
	Timestamp        *timestamppb.Timestamp
	Source           string
	Capability       string
	TrackID          int32
	ClassName        string
	Confidence       float32
	X1, Y1, X2, Y2   float32
	OCRText          string
	OCRConfidence    float32
	PlateX1, PlateY1 float32
	PlateX2, PlateY2 float32
	ProcessingMs     float64
	VehicleClass     string
	MakeModel        string
	Color            string
	SceneL1          string
	SceneL2          string
	Attributes       map[string]string
	Labels           map[string]string
}

func ResultToRow(r ResultFields, seq uint64) persistence.ResultRow {
	attrs := r.Attributes
	if attrs == nil {
		attrs = map[string]string{}
	}
	labels := r.Labels
	if labels == nil {
		labels = map[string]string{}
	}

	// Confidence: explicit field, else OCR confidence for alpr, else 0.
	conf := r.Confidence
	if conf == 0 && r.OCRConfidence > 0 {
		conf = r.OCRConfidence
	}

	// Prefer denormalized columns; fall back to attributes map.
	makeModel := r.MakeModel
	if makeModel == "" {
		makeModel = attrs["make_model"]
	}
	if makeModel == "" {
		if m, ok := attrs["make"]; ok {
			makeModel = m
		}
		if m, ok := attrs["model"]; ok {
			if makeModel != "" {
				makeModel += " " + m
			} else {
				makeModel = m
			}
		}
	}
	color := r.Color
	if color == "" {
		color = attrs["color"]
	}
	vc := r.VehicleClass
	if vc == "" {
		vc = attrs["vehicle_class"]
	}
	if vc == "" {
		vc = r.ClassName // parent object class as last resort
	}
	sceneL1 := r.SceneL1
	if sceneL1 == "" {
		sceneL1 = labels["scene_l1"]
	}
	sceneL2 := r.SceneL2
	if sceneL2 == "" {
		sceneL2 = labels["scene_l2"]
	}

	return persistence.ResultRow{
		FrameID: r.FrameID, TS: ProtoTS(r.Timestamp), Source: r.Source,
		Capability: r.Capability, TrackID: r.TrackID, ClassName: r.ClassName,
		Confidence: conf, X1: r.X1, Y1: r.Y1, X2: r.X2, Y2: r.Y2,
		OCRText: r.OCRText, OCRConfidence: r.OCRConfidence,
		PlateX1: r.PlateX1, PlateY1: r.PlateY1, PlateX2: r.PlateX2, PlateY2: r.PlateY2,
		ProcessingMs: r.ProcessingMs, VehicleClass: vc, MakeModel: makeModel, Color: color,
		SceneL1: sceneL1, SceneL2: sceneL2, NatsSeq: seq,
		Attributes: attrs, Labels: labels,
	}
}

// SceneFields ← scene.v1.SceneResult
type SceneFields struct {
	FrameID           int64
	Timestamp         *timestamppb.Timestamp
	Source            string
	Level1            string
	Level2            string
	Level1Confidence  float32
	Level2Confidence  float32
	Specialists       []string
	Backend           string
	TemporalRequested bool
	Notes             string
}

func SceneToRow(s SceneFields, seq uint64) persistence.SceneRow {
	specs := s.Specialists
	if specs == nil {
		specs = []string{}
	}
	return persistence.SceneRow{
		FrameID: s.FrameID, TS: ProtoTS(s.Timestamp), Source: s.Source,
		Level1: s.Level1, Level2: s.Level2,
		Level1Confidence: s.Level1Confidence, Level2Confidence: s.Level2Confidence,
		Specialists: specs, Backend: s.Backend,
		TemporalRequested: BoolU8(s.TemporalRequested), Notes: s.Notes,
		NatsSeq: seq,
	}
}
