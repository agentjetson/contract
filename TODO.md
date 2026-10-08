# Integrating pkg/persistence + clickhouse-consumer

## Layout in agentjetson/contract

```
contract/
├── pkg/
│   └── persistence/          # shared CH client + batch writers
├── gen/go/                   # buf-generated protos (make generate / Docker)
├── clickhouse-consumer/      # JetStream pull → persistence
├── object-storage/           # uses persistence.InsertObjectMeta
├── query-service/            # SELECTs; ApplySchema default false
├── aggregator/               # Go correlator, durables agg-objects / agg-results
└── seed/sql/                 # sole schema owner
```

## Done

- [x] pkg/persistence API (InsertObjects/Results/Scenes/Detections/Transcripts/ObjectMeta)
- [x] clickhouse-consumer protobuf decode (Docker runs `buf generate`)
- [x] object-storage → persistence.InsertObjectMeta (+ MemoryStore DEMO_MODE)
- [x] taxonomy single-source: contract/domain/taxonomy.yaml (scene-router aligned)
- [x] **core cutover** — residual only (demo Alert `consumer`, `video_server`, `video_viewer`)
  - [x] No ClickHouse service in core (uses contract)
  - [x] No C++ clickhouse-consumer / aggregator in core
  - [x] No inline `CREATE TABLE` in core (read-only SELECTs on `cv_detections`)
  - [x] `CLICKHOUSE_*` / `NATS_URL` point at contract stack
  - [x] JetStream streams match `domain/nats-subjects.yaml`
  - [x] Aggregator durables `agg-objects` / `agg-results` (contract/aggregator)

## Remaining

### query-service

1. Prefer `persistence.Open` + `ch.Conn()` for SELECTs against
   `query_cv_*` / `query_audio_transcripts` views (`003_views.sql`).
2. `APPLY_SCHEMA` already defaults to `false`.
3. Same `replace` for pkg/persistence (already in go.mod).

### Known compose friction (contract)

- MinIO and ClickHouse both default host port **9000** — remap MinIO API port if binding both on localhost.
- `voice-query-service` name vs `query-service` directory — rename for consistency when convenient.
- `core_net` external network assumption — core compose now uses `agentjetson-contract_default`.
