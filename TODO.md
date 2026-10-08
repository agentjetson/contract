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
├── clickhouse-consumer/      # JetStream pull → persistence
│   ├── cmd/consumer/main.go
│   ├── internal/...
│   ├── Dockerfile
│   └── go.mod
├── object-storage/           # already Go — switch metadata inserts here
├── query-service/            # already Go — switch SELECTs / drop ApplySchema
└── seed/sql/                 # sole schema owner
```

## object-storage

Replace local metadata store ClickHouse path with:

```go
import "github.com/agentjetson/contract/pkg/persistence"

ch, err := persistence.Open(persistence.Config{...})
err = ch.InsertObjectMeta(ctx, persistence.ObjectMetaRow{
    ObjectID: r.ObjectID,
    Kind:     r.Kind,
    // ...
})
```

Keep `MemoryStore` for `DEMO_MODE`.

In `object-storage/go.mod`:

```
require github.com/agentjetson/contract/pkg/persistence v0.0.0
replace github.com/agentjetson/contract/pkg/persistence => ../pkg/persistence
```

## query-service

1. Delete `ApplySchema` (WRITE_SPEC / CONSUMING.md).
2. Open via `persistence.Open` and use `ch.Conn()` for SELECTs against
   `query_cv_results`, `query_cv_objects`, `query_cv_scenes`,
   `query_cv_detections`, `query_audio_transcripts` (see `003_views.sql`).
3. Same `replace` directive as above.

## docker-compose.yml (contract) snippet

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

## core changes checklist

- [ ] Remove `clickhouse` service (use contract's).
- [ ] Remove `clickhouse-consumer` C++ binary / CMake target.
- [ ] Drop inline `CREATE TABLE` from any remaining C++ paths.
- [ ] Set `CLICKHOUSE_HOST` for video_server etc. to the contract host.
- [ ] Ensure JetStream streams `CV_EVENTS`, `CV_ALERTS`, `AUDIO_EVENTS`
      match `domain/nats-subjects.yaml`.
