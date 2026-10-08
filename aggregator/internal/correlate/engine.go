package correlate

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	detectionv1 "github.com/agentjetson/core/gen/go/detection/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// WatchEntry is a single watchlist rule.
type WatchEntry struct {
	Label   string
	MinConf float32
}

// DefaultWatchlist mirrors the former C++ pipeline defaults.
func DefaultWatchlist() []WatchEntry {
	return []WatchEntry{
		{Label: "person", MinConf: 0.55},
		{Label: "car", MinConf: 0.50},
		{Label: "truck", MinConf: 0.50},
		{Label: "AF29KX", MinConf: 0.60}, // example plate
	}
}

// ParseWatchlist parses "label:conf,label2:conf2" or falls back to defaults.
func ParseWatchlist(raw string) []WatchEntry {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultWatchlist()
	}
	parts := strings.Split(raw, ",")
	out := make([]WatchEntry, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		label, confStr, ok := strings.Cut(p, ":")
		entry := WatchEntry{Label: strings.TrimSpace(label), MinConf: 0.5}
		if ok {
			if f, err := strconv.ParseFloat(strings.TrimSpace(confStr), 32); err == nil {
				entry.MinConf = float32(f)
			}
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return DefaultWatchlist()
	}
	return out
}

type trackKey struct {
	FrameID int64
	TrackID int32
}

type pendingObject struct {
	obj      *detectionv1.ObjectEnvelope
	ocrText  string
	ocrConf  float32
	plateBox *detectionv1.BoundingBox
	hasALPR  bool
	created  time.Time
}

// Engine correlates ObjectEnvelope + CapabilityResult by (frame_id, track_id)
// and emits Alerts. Thread-safe.
type Engine struct {
	mu         sync.Mutex
	pending    map[trackKey]*pendingObject
	watchlist  []WatchEntry
	emitAfter  time.Duration
	pruneAfter time.Duration
	publish    func(subject string, data []byte) error
}

// New builds an Engine. publish is called with subject "cv.alert" and protobuf bytes.
func New(watchlist []WatchEntry, emitAfter, pruneAfter time.Duration, publish func(subject string, data []byte) error) *Engine {
	return &Engine{
		pending:    make(map[trackKey]*pendingObject),
		watchlist:  watchlist,
		emitAfter:  emitAfter,
		pruneAfter: pruneAfter,
		publish:    publish,
	}
}

// OnObject handles a cv.object.* message body.
func (e *Engine) OnObject(data []byte) {
	var obj detectionv1.ObjectEnvelope
	if err := proto.Unmarshal(data, &obj); err != nil {
		slog.Warn("decode ObjectEnvelope", "err", err, "bytes", len(data))
		return
	}
	key := trackKey{FrameID: obj.FrameId, TrackID: obj.TrackId}
	e.mu.Lock()
	defer e.mu.Unlock()
	p, ok := e.pending[key]
	if !ok {
		p = &pendingObject{created: time.Now()}
		e.pending[key] = p
	}
	p.obj = &obj
	p.created = time.Now()
	if p.hasALPR {
		e.emitLocked(key, p)
	}
}

// OnResult handles a cv.result.* message body (currently ALPR).
func (e *Engine) OnResult(data []byte) {
	var res detectionv1.CapabilityResult
	if err := proto.Unmarshal(data, &res); err != nil {
		slog.Warn("decode CapabilityResult", "err", err, "bytes", len(data))
		return
	}
	if res.Capability != "alpr" {
		return
	}
	key := trackKey{FrameID: res.FrameId, TrackID: res.TrackId}
	e.mu.Lock()
	defer e.mu.Unlock()
	p, ok := e.pending[key]
	if !ok {
		p = &pendingObject{created: time.Now()}
		e.pending[key] = p
	}
	p.ocrText = res.OcrText
	p.ocrConf = res.OcrConfidence
	p.plateBox = res.PlateBox
	p.hasALPR = true
	p.created = time.Now()
	if p.obj != nil && p.obj.FrameId != 0 {
		e.emitLocked(key, p)
	}
}

// Tick emits timed-out pending objects (no ALPR) and prunes stale entries.
func (e *Engine) Tick() {
	now := time.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	for key, p := range e.pending {
		age := now.Sub(p.created)
		if age > e.pruneAfter {
			delete(e.pending, key)
			continue
		}
		if age > e.emitAfter && p.obj != nil {
			e.emitLocked(key, p)
		}
	}
}

func (e *Engine) emitLocked(key trackKey, p *pendingObject) {
	if p.obj == nil {
		delete(e.pending, key)
		return
	}
	alert := e.buildAlert(p)
	payload, err := proto.Marshal(alert)
	if err != nil {
		slog.Warn("marshal Alert", "err", err)
		delete(e.pending, key)
		return
	}
	if err := e.publish("cv.alert", payload); err != nil {
		slog.Warn("publish alert", "err", err)
	} else {
		slog.Info("ALERT",
			"frame", alert.FrameId,
			"track", p.obj.TrackId,
			"class", p.obj.ClassName,
			"plate", p.ocrText,
			"hit", alert.WatchlistHit,
			"match", alert.MatchedLabel,
		)
	}
	delete(e.pending, key)
}

func (e *Engine) buildAlert(p *pendingObject) *detectionv1.Alert {
	o := p.obj
	alert := &detectionv1.Alert{
		FrameId:      o.FrameId,
		Timestamp:    timestamppb.Now(),
		Source:       o.Source,
		E2ELatencyMs: o.CaptureLatencyMs,
		WatchlistHit: false,
		MatchedLabel: "",
	}

	det := &detectionv1.Detection{
		ClassId:    o.ClassId,
		ClassName:  o.ClassName,
		Confidence: o.Confidence,
		Box:        o.Box,
		TrackId:    o.TrackId,
	}
	if p.hasALPR {
		det.OcrText = p.ocrText
		det.OcrConfidence = p.ocrConf
	}
	alert.Detections = append(alert.Detections, det)

	if p.hasALPR && p.ocrText != "" {
		pd := &detectionv1.Detection{
			ClassId:       1000,
			ClassName:     "license_plate",
			Confidence:    p.ocrConf,
			Box:           p.plateBox,
			TrackId:       o.TrackId,
			OcrText:       p.ocrText,
			OcrConfidence: p.ocrConf,
		}
		alert.Detections = append(alert.Detections, pd)
	}

	matched, hit := e.checkWatchlist(p)
	alert.WatchlistHit = hit
	alert.MatchedLabel = matched
	return alert
}

func (e *Engine) checkWatchlist(p *pendingObject) (matched string, hit bool) {
	o := p.obj
	for _, w := range e.watchlist {
		if o.ClassName == w.Label && o.Confidence >= w.MinConf {
			matched = o.ClassName
			if o.TrackId >= 0 {
				matched = fmt.Sprintf("%s#%d", matched, o.TrackId)
			}
			return matched, true
		}
	}
	if p.hasALPR && p.ocrText != "" {
		for _, w := range e.watchlist {
			if p.ocrText == w.Label && p.ocrConf >= w.MinConf {
				matched = "plate:" + p.ocrText
				if o.TrackId >= 0 {
					matched = fmt.Sprintf("%s#%d", matched, o.TrackId)
				}
				return matched, true
			}
		}
	}
	return "", false
}
