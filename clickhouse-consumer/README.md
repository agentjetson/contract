# clickhouse-consumer (Go)

NATS JetStream → ClickHouse writer for AgentJetson.

Replaces `core/src/clickhouse_consumer/main.cpp`. Schema is owned by
`contract/seed/sql` (`make schema`); this service **never** runs `CREATE TABLE`.

## Subjects → tables

| Subject | Table |
|---------|--------|
| `cv.object.>` | `cv_objects` |
| `cv.result.>` | `cv_results` |
| `cv.scene.>` | `cv_scenes` |
| `cv.alert` | `cv_detections` |
| `audio.transcript` | `audio_transcripts` |

See `seed/WRITE_SPEC.md`.

## Shared package

Writes go through `github.com/agentjetson/contract/pkg/persistence`.
object-storage and query-service should import the same module for
`object_meta` inserts and `query_*` SELECTs.

## Env

| Variable | Default |
|----------|---------|
| `NATS_URL` | `nats://localhost:4222` |
| `CLICKHOUSE_HOST` | `localhost` |
| `CLICKHOUSE_PORT` | `9000` |
| `CLICKHOUSE_USER` | `default` |
| `CLICKHOUSE_PASSWORD` | `pass` |
| `CLICKHOUSE_DB` | `default` |
| `FLUSH_MS` | `500` |
| `FLUSH_SIZE` | `32` |
| `STREAM_EVENTS` | `CV_EVENTS` |
| `STREAM_ALERTS` | `CV_ALERTS` |
| `STREAM_AUDIO` | `AUDIO_EVENTS` |

## Run

```bash
# from contract root after make up && make schema
cd clickhouse-consumer
go mod tidy
export NATS_URL=nats://localhost:4222
export CLICKHOUSE_HOST=localhost
go run ./cmd/consumer
```

## Proto wiring

`cmd/consumer/main.go` currently uses lightweight local message shapes so the
tree builds without generated stubs. After `make generate` in the contract
root, replace them with:

```go
detectionv1 "github.com/agentjetson/contract/gen/go/detection/v1"
audiov1     "github.com/agentjetson/contract/gen/go/audio/v1"
scenev1     "github.com/agentjetson/contract/gen/go/scene/v1"
```

and call `proto.Unmarshal` into the generated types, mapping via
`internal/map`.

## Core cutover

1. Point core compose `CLICKHOUSE_HOST` at the contract ClickHouse instance.
2. Remove the C++ `clickhouse` + `clickhouse-consumer` services from core.
3. Drop `BUILD_CLICKHOUSE_CONSUMER` / inline `CREATE TABLE` in C++.
