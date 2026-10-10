package otel

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/trace"
)

// SetupLogging installs the default slog handler.
//
// When serviceName is non-empty (normal Init path), records go to:
//  1. stdout as JSON (with trace_id/span_id when ctx has a span)
//  2. OTLP logs via the global LoggerProvider (collector → Loki)
//
// When serviceName is empty (OTEL_SDK_DISABLED), only stdout is used.
//
// Prefer slog.InfoContext / ErrorContext so handlers can see the span:
//
//	slog.InfoContext(ctx, "ingested alert", "frame", id)
func SetupLogging(serviceName string) {
	var stdout slog.Handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	stdout = &traceHandler{inner: stdout}

	if serviceName == "" {
		slog.SetDefault(slog.New(stdout))
		return
	}

	// Bridges slog → OTEL Logs API; correlates via span in ctx automatically.
	otelH := otelslog.NewHandler(serviceName)

	slog.SetDefault(slog.New(&multiHandler{handlers: []slog.Handler{stdout, otelH}}))
}

// multiHandler fans a record out to several handlers (stdout + OTLP).
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var first error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		// Clone so each handler can safely add attrs / consume the record.
		if err := h.Handle(ctx, r.Clone()); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: next}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: next}
}

// traceHandler wraps an slog.Handler and adds trace_id / span_id attrs from ctx.
type traceHandler struct {
	inner slog.Handler
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{inner: h.inner.WithGroup(name)}
}

// TraceID returns the current trace id from ctx, or "" if none.
func TraceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.TraceID().String()
}

// SpanID returns the current span id from ctx, or "" if none.
func SpanID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return ""
	}
	return sc.SpanID().String()
}
