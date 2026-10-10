package otel

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Header keys follow the W3C Trace Context / OpenTelemetry messaging convention.
const (
	HeaderTraceParent = "traceparent"
	HeaderTraceState  = "tracestate"
	HeaderBaggage     = "baggage"
)

// NATSHeaderCarrier adapts nats.Header to propagation.TextMapCarrier.
type NATSHeaderCarrier nats.Header

func (c NATSHeaderCarrier) Get(key string) string {
	if c == nil {
		return ""
	}
	return nats.Header(c).Get(key)
}

func (c NATSHeaderCarrier) Set(key, value string) {
	if c == nil {
		return
	}
	nats.Header(c).Set(key, value)
}

func (c NATSHeaderCarrier) Keys() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	return out
}

// InjectMsg writes the current span context from ctx into msg.Header
// (creating the map if needed) using the global TextMapPropagator.
func InjectMsg(ctx context.Context, msg *nats.Msg) {
	if msg == nil {
		return
	}
	if msg.Header == nil {
		msg.Header = make(nats.Header)
	}
	otel.GetTextMapPropagator().Inject(ctx, NATSHeaderCarrier(msg.Header))
}

// ExtractMsg returns a context with the remote span context from msg headers.
// If headers are missing or invalid, ctx is returned unchanged (aside from
// being derived from parent).
func ExtractMsg(parent context.Context, msg *nats.Msg) context.Context {
	if msg == nil || msg.Header == nil {
		return parent
	}
	return otel.GetTextMapPropagator().Extract(parent, NATSHeaderCarrier(msg.Header))
}

// StartConsumerSpan extracts trace context from msg and starts a CONSUMER span.
// Caller must End the returned span.
func StartConsumerSpan(parent context.Context, tracerName, spanName string, msg *nats.Msg) (context.Context, trace.Span) {
	ctx := ExtractMsg(parent, msg)
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.operation", "receive"),
	}
	if msg != nil {
		attrs = append(attrs, attribute.String("messaging.destination", msg.Subject))
	}
	return Tracer(tracerName).Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attrs...),
	)
}

// StartProducerSpan starts a PRODUCER span and injects it into msg headers.
// Caller must End the span after PublishMsg succeeds or fails.
func StartProducerSpan(ctx context.Context, tracerName, spanName string, msg *nats.Msg) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("messaging.system", "nats"),
		attribute.String("messaging.operation", "publish"),
	}
	if msg != nil {
		attrs = append(attrs, attribute.String("messaging.destination", msg.Subject))
	}
	ctx, span := Tracer(tracerName).Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(attrs...),
	)
	InjectMsg(ctx, msg)
	return ctx, span
}

// SpanFromContext returns the SpanContext in ctx (may be invalid).
func SpanFromContext(ctx context.Context) trace.SpanContext {
	return trace.SpanContextFromContext(ctx)
}

// ContextWithRemote returns a context that carries sc as the remote parent.
func ContextWithRemote(parent context.Context, sc trace.SpanContext) context.Context {
	if !sc.IsValid() {
		return parent
	}
	return trace.ContextWithRemoteSpanContext(parent, sc)
}

// RecordError marks the span as error if err != nil.
func RecordError(span trace.Span, err error) {
	if err == nil || !span.IsRecording() {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// Ensure propagation package is referenced when only nats helpers are imported.
var _ propagation.TextMapCarrier = NATSHeaderCarrier(nil)
