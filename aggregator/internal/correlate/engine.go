package correlate

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	detectionv1 "github.com/agentjetson/core/gen/go/detection/v1"
	"github.com/agentjetson/core/pkg/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// WatchEntry is a single watchlist rule.
type WatchEntry struct {
	Label   string
	MinConf float32
}

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
	// parentSC is the remote span from the latest contributing NATS message
	// so emit.alert can continue the same trace_id.
	parentSC trace.SpanContext
}

// Engine correlates ObjectEnvelope + CapabilityResult by (frame_id, track_id)
// and emits Alerts. Thread-safe.
type Engine struct {
	mu         sync.Mutex
	pending    map[trackKey]*pendingObject
	watchlist  []WatchEntry
	emitAfter  time.Duration
	pruneAfter time.Duration
	publish    func(ctx context.Context, subject string, data []byte) error
}

// New builds an Engine. publish is called with subject "cv.alert" and protobuf bytes.
func New(watchlist []WatchEntry, emitAfter, pruneAfter time.Duration, publish func(ctx context.Context, subject string, data []byte) error) *Engine {
	return &Engine{
		pending:    make(map[trackKey]*pendingObject),
		watchlist:  watchlist,
		emitAfter:  emitAfter,
		pruneAfter: pruneAfter,
		publish:    publish,
	}
}

// OnObject handles a cv.object.* message body. ctx should carry the extracted
// NATS span (consumer span) so emit can join the upstream trace.
func (e *Engine) OnObject(ctx context.Context, data []byte) {
	var obj detectionv1.ObjectEnvelope
	if err := proto.Unmarshal(data, &obj); err != nil {
		slog.WarnContext(ctx, "decode ObjectEnvelope", "err", err, "bytes", len(data))
		return
	}
	key := trackKey{FrameID: obj.FrameId, TrackID: obj.TrackId}
	sc := otel.SpanFromContext(ctx)
	e.mu.Lock()
	defer e.mu.Unlock()
	p, ok := e.pending[key]
	if !ok {
		p = &pendingObject{created: time.Now()}
		e.pending[key] = p
	}
	p.obj = &obj
	p.created = time.Now()
	if sc.IsValid() {
		p.parentSC = sc
	}
	if p.hasALPR {
		e.emitLocked(key, p)
	}
}

// OnResult handles a cv.result.* message body (currently ALPR).
func (e *Engine) OnResult(ctx context.Context, data []byte) {
	var res detectionv1.CapabilityResult
	if err := proto.Unmarshal(data, &res); err != nil {
		slog.WarnContext(ctx, "decode CapabilityResult", "err", err, "bytes", len(data))
		return
	}
	if res.Capability != "alpr" {
		return
	}
	key := trackKey{FrameID: res.FrameId, TrackID: res.TrackId}
	sc := otel.SpanFromContext(ctx)
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
	if sc.IsValid() {
		p.parentSC = sc
	}
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

	ctx := otel.ContextWithRemote(context.Background(), p.parentSC)
	ctx, span := otel.Tracer("aggregator").Start(ctx, "emit.alert",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	if err := e.publish(ctx, "cv.alert", payload); err != nil {
		otel.RecordError(span, err)
		slog.WarnContext(ctx, "publish alert", "err", err)
	} else {
		slog.InfoContext(ctx, "ALERT",
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
