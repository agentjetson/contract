# Observability (LGT)

Local **Loki + Grafana + Tempo** stack for AgentJetson core.

```
Go services  --OTLP/gRPC-->  otel-collector  --traces-->  Tempo
                              |               --logs---->  Loki
                              v
                           Grafana (:3000)  Explore + dashboards
```

## Bring-up

```bash
make up   # or: docker compose up -d
```

| UI | URL | Credentials |
|----|-----|-------------|
| **Grafana** | http://localhost:3000 | `admin` / `admin` |
| Tempo API | http://localhost:3200 | — |
| Loki API | http://localhost:3100 | — |

Datasources are auto-provisioned from `grafana/provisioning/datasources/`.

## End-to-end `trace_id` flow

```
edge → ingest (gRPC span)
         → nats-publisher (gRPC child + nats.publish PRODUCER)
              ─ NATS header: traceparent ─→ JetStream
              ↓
   aggregator / clickhouse-consumer / scene-gate
         (CONSUMER span via ExtractMsg)
         aggregator emit.alert continues parent → cv.alert (+ inject)
```

1. **gRPC** — `otel.GRPCServerOption` / `GRPCDialOptions` on ingest ↔ nats-publisher and scene-gate → ingest.
2. **NATS headers** — `pkg/otel.InjectMsg` / `ExtractMsg` (W3C `traceparent` / `tracestate`).
3. **Logs** — `slog.InfoContext` + SetupLogging adds `trace_id` / `span_id` JSON fields; Grafana derived field jumps to Tempo.

### Example queries

**Loki**
```logql
{service_name=~"ingest|nats-publisher|aggregator|clickhouse-consumer"} | json | trace_id != ""
```

**Tempo (TraceQL)**
```
{ resource.service.name = "ingest" }
{ name =~ "nats.publish.*" }
{ name = "emit.alert" }
```

## Disable telemetry

```bash
OTEL_SDK_DISABLED=true docker compose up -d
```
