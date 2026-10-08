# Integrating pkg/persistence + clickhouse-consumer

## Layout in agentjetson/contract

```
contract/
├── pkg/
│   └── persistence/          # shared CH client + batch writers
│       ├── client.go
│       ├── rows.go
│       ├── writer.go
│       ├── time.go
│       └── go.mod
├── gen/go/                   # buf-generated protos (make generate)
├── clickhouse-consumer/      # JetStream pull → persistence
│   ├── cmd/consumer/main.go  # protobuf decode wired
│   ├── internal/...
│   ├── Dockerfile
│   └── go.mod
├── object-storage/           # uses persistence.InsertObjectMeta
├── query-service/            # SELECTs via persistence; no ApplySchema in prod
└── seed/sql/                 # sole schema owner
```

## Done

- [x] pkg/persistence API (InsertObjects/Results/Scenes/Detections/Transcripts/ObjectMeta)
- [x] clickhouse-consumer protobuf decode via gen/go
- [x] object-storage → persistence.InsertObjectMeta (+ MemoryStore DEMO_MODE)
- [x] taxonomy single-source: contract/domain/taxonomy.yaml (scene-router aligned)

## Remaining

### query-service

1. Prefer `persistence.Open` + `ch.Conn()` for SELECTs against
   `query_cv_results`, `query_cv_objects`, `query_cv_scenes`,
   `query_cv_detections`, `query_audio_transcripts` (see `003_views.sql`).
2. Default `ApplySchema=false` (schema owned by seed/sql only).
3. Same `replace` directive for pkg/persistence.

### docker-compose.yml (contract) snippet

```yaml
  clickhouse-consumer:
    build:
      context: .
      dockerfile: clickhouse-consumer/Dockerfile
    environment:
      NATS_URL: ${NATS_URL:-nats://host.docker.internal:4222}
      CLICKHOUSE_HOST: clickhouse
      CLICKHOUSE_PORT: "9000"
      CLICKHOUSE_USER: default
      CLICKHOUSE_PASSWORD: pass
      CLICKHOUSE_DB: default
    depends_on:
      clickhouse:
        condition: service_healthy
    restart: unless-stopped
```

NATS still lives in core compose (or a shared stack). Point `NATS_URL`
at that instance.

### core changes checklist

- [ ] Remove `clickhouse` service (use contract's).
- [ ] Remove `clickhouse-consumer` C++ binary / CMake target.
- [ ] Drop inline `CREATE TABLE` from any remaining C++ paths.
- [ ] Set `CLICKHOUSE_HOST` for video_server etc. to the contract host.
- [ ] Ensure JetStream streams `CV_EVENTS`, `CV_ALERTS`, `AUDIO_EVENTS`
      match `domain/nats-subjects.yaml`.

### aggregator (Go)

New service under `contract/aggregator/`. Pure NATS correlator — no CH writes.

```yaml
  aggregator:
    build:
      context: .
      dockerfile: aggregator/Dockerfile
    environment:
      NATS_URL: ${NATS_URL:-nats://host.docker.internal:4222}
      STREAM_EVENTS: CV_EVENTS
      STREAM_ALERTS: CV_ALERTS
      EMIT_TIMEOUT_MS: "800"
    restart: unless-stopped
```

Core cutover:
- [ ] Swap core compose `aggregator` image/command to this binary
- [ ] Remove `core/src/aggregator` + CMake target
- [ ] Keep durable names `agg-objects` / `agg-results` for zero-downtime consumer continuity
