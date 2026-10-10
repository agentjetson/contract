# pkg/otel

Shared OpenTelemetry bootstrap for AgentJetson Go microservices.

**Traces** export over OTLP/gRPC to `otel-collector` → **Tempo**.  
**Logs** export over OTLP/gRPC to `otel-collector` → **Loki** (native OTLP),
and are still written as JSON to stdout for `docker logs`.

When a log call uses a context with an active span, OTLP records are correlated
to that trace; stdout JSON also carries `trace_id` / `span_id`.

## Usage

```go
ctx := context.Background()
shutdown, err := otel.Init(ctx, "ingest")
if err != nil {
    slog.Error("otel", "err", err)
    os.Exit(1)
}
defer func() { _ = shutdown(context.Background()) }()

// Prefer context-aware logging so Loki/Tempo can join on the same trace:
sctx, span := otel.StartSpan(ctx, "ingest", "IngestAlert")
defer span.End()
slog.InfoContext(sctx, "ingested alert", "frame", id)
```

## Environment

| Variable | Default | Meaning |
|----------|---------|---------|
| `OTEL_SERVICE_NAME` | argument to `Init` | Resource `service.name` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `otel-collector:4317` | Collector host:port |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | Skip TLS (local stack) |
| `OTEL_SDK_DISABLED` | unset | Set `true` to no-op |

## LGT UI

| Signal | Backend | Grafana |
|--------|---------|---------|
| Traces | Tempo `:3200` | Explore → Tempo |
| Logs | Loki `:3100` | Explore → Loki (default) |
| UI | Grafana `:3000` | admin / admin |

Click `trace_id` on a Loki log line to open the Tempo waterfall.
