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

## Following a request end-to-end

1. **gRPC path (best correlation)**  
   Edge → `ingest` → `nats-publisher` shares one W3C `traceparent`.  
   Both services use `otel.GRPCServerOption` / `GRPCDialOptions`, so Tempo shows a single trace with child spans.

2. **Logs with `trace_id`**  
   `pkg/otel.SetupLogging` (called from `Init`) adds `trace_id` and `span_id` to JSON slog lines when you use `slog.InfoContext(ctx, ...)`.  
   In Grafana Explore → Loki, click the derived **View Trace** field to jump to Tempo.

3. **Example Loki queries**
   ```logql
   {service_name="ingest"} |= "ingested"
   {service_name=~"ingest|nats-publisher"} | json | trace_id != ""
   {service_name="clickhouse-consumer"} |= "ERROR"
   ```

4. **Tempo TraceQL**
   ```
   { resource.service.name = "ingest" }
   { name = "IngestAlert" }
   ```

## NATS consumers (aggregator, clickhouse-consumer, scene-gate)

These services process messages asynchronously. Spans they create today are **not** automatically linked to the ingest-side `trace_id` unless the publisher embeds trace context in NATS headers (future work). You can still filter by `service_name` and time window in Grafana.

## Disable telemetry

```bash
OTEL_SDK_DISABLED=true docker compose up -d
```
