# Integrating pkg/persistence + clickhouse-consumer

## Layout in agentjetson/core

```
core/
├── pkg/
│   └── persistence/          # shared CH client + batch writers
├── gen/go/                   # buf-generated protos (make generate / Docker)
├── clickhouse-consumer/      # JetStream pull → persistence
├── object-storage/           # uses persistence.InsertObjectMeta
├── query-service/            # SELECTs via persistence on query_* views
├── aggregator/               # Go correlator, durables agg-objects / agg-results
└── seed/sql/                 # sole schema owner
```

## Done

- [x] pkg/persistence API (InsertObjects/Results/Scenes/Detections/Transcripts/ObjectMeta)
- [x] clickhouse-consumer protobuf decode (Docker runs `buf generate`)
- [x] object-storage → persistence.InsertObjectMeta (+ MemoryStore DEMO_MODE)
- [x] taxonomy single-source: core/domain/taxonomy.yaml (scene-router aligned)
- [x] **core cutover** — residual only (demo Alert `consumer`, `video_server`, `video_viewer`)
- [x] **query-service**
  - [x] `persistence.Open` + `Conn()` for SELECTs
  - [x] Query `query_cv_*` / `query_audio_transcripts` views only
  - [x] No ApplySchema / no CREATE TABLE
  - [x] Production HTTP entrypoint wired to tools.Server

## Known compose friction

- MinIO and ClickHouse both default host port **9000** — remap MinIO API if binding both on localhost.
- Build query-service from monorepo root: `docker build -f query-service/Dockerfile .`
