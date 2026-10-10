// Package otel provides shared OpenTelemetry bootstrap for AgentJetson Go services.
//
// Call Init once at process start; defer the returned shutdown. When
// OTEL_SDK_DISABLED=true the SDK is a no-op so services keep working without
// a collector.
//
// Environment (standard OTEL vars):
//
//	OTEL_SERVICE_NAME              — overrides the name passed to Init
//	OTEL_EXPORTER_OTLP_ENDPOINT    — host:port (default otel-collector:4317)
//	OTEL_EXPORTER_OTLP_INSECURE    — "true" to skip TLS (default true for local)
//	OTEL_SDK_DISABLED              — "true" disables the SDK entirely
package otel

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

// Init configures the global TracerProvider, LoggerProvider, and TextMapPropagator.
// serviceName is used when OTEL_SERVICE_NAME is unset.
// The returned shutdown flushes exporters; call it on process exit.
//
// Init also installs SetupLogging() so slog records go to stdout JSON and to
// the OTLP log pipeline (collector → Loki). Prefer slog.*Context so trace_id
// is correlated when a span is active.
func Init(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	noop := func(context.Context) error { return nil }

	if disabled() {
		SetupLogging("") // stdout only
		slog.Info("otel disabled (OTEL_SDK_DISABLED)")
		return noop, nil
	}

	if v := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); v != "" {
		serviceName = v
	}
	if serviceName == "" {
		serviceName = "agentjetson"
	}

	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		endpoint = "otel-collector:4317"
	}
	// Strip scheme if a full URL was provided.
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	insecure := true
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"))); v == "false" || v == "0" {
		insecure = false
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
	)
	if err != nil {
		return noop, fmt.Errorf("otel resource: %w", err)
	}

	// --- traces ---
	traceOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
	}
	if insecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}
	traceExp, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return noop, fmt.Errorf("otel otlp trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// --- logs ---
	logOpts := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(endpoint),
	}
	if insecure {
		logOpts = append(logOpts, otlploggrpc.WithInsecure())
	}
	logExp, err := otlploggrpc.New(ctx, logOpts...)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return noop, fmt.Errorf("otel otlp log exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	// slog → stdout JSON + OTLP (after LoggerProvider is global)
	SetupLogging(serviceName)

	slog.Info("otel initialized",
		"service", serviceName,
		"endpoint", endpoint,
		"insecure", insecure,
		"signals", "traces+logs",
	)

	return func(ctx context.Context) error {
		var first error
		if err := lp.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
		if err := tp.Shutdown(ctx); err != nil && first == nil {
			first = err
		}
		return first
	}, nil
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// StartSpan is a convenience wrapper around Tracer(name).Start.
func StartSpan(ctx context.Context, tracerName, spanName string) (context.Context, trace.Span) {
	return Tracer(tracerName).Start(ctx, spanName)
}

// GRPCServerOption returns a grpc.ServerOption that enables OTEL stats
// handling for inbound RPCs (spans + propagation).
func GRPCServerOption() grpc.ServerOption {
	return grpc.StatsHandler(otelgrpc.NewServerHandler())
}

// GRPCDialOptions returns dial options for outbound gRPC clients so that
// spans are created and context is propagated.
func GRPCDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	}
}

// HTTPHandler wraps h with otelhttp instrumentation.
// operation is the span name (e.g. "http.server").
func HTTPHandler(operation string, h http.Handler) http.Handler {
	return otelhttp.NewHandler(h, operation)
}

func disabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_SDK_DISABLED")))
	return v == "true" || v == "1"
}
