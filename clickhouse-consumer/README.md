# clickhouse-consumer (Go)

NATS JetStream → ClickHouse writer for AgentJetson.

Schema is owned by `core/seed/sql` (`make schema`); this service **never**
runs `CREATE TABLE`.

## Subjects → tables

| Subject | Proto | Table | Rows |
|---------|-------|-------|------|
| `cv.object.>` | `detection.v1.ObjectEnvelope` | `cv_objects` | 1 |
| `cv.result.>` | `detection.v1.CapabilityResult` | `cv_results` | 1 |
| `cv.scene.>` | `scene.v1.SceneResult` | `cv_scenes` | 1 |
| `cv.alert` | `detection.v1.Alert` | `cv_detections` | 1 per Detection |
| `audio.transcript` | `audio.v1.Transcript` | `audio_transcripts` | 1 |

JPEG / crop bytes are never stored. See `seed/WRITE_SPEC.md`.

## Reliability

- Decode via `proto.Unmarshal` into generated gen/go types.
- Map through `internal/map` → `pkg/persistence` row types.
- **Ack only after successful `Insert*` flush.** On insert failure messages are
  `Nak`'d and redelivered. Poison-pill decode errors are Ack'd so they do not
  block the durable.

## Durables

| Durable | Stream | Filter |
|---------|--------|--------|
| `ch-consumer-objects` | `CV_EVENTS` | `cv.object.>` |
| `ch-consumer-results` | `CV_EVENTS` | `cv.result.>` |
| `ch-consumer-scenes` | `CV_EVENTS` | `cv.scene.>` |
| `ch-consumer-alerts` | `CV_ALERTS` | `cv.alert` |
| `ch-consumer-audio` | `AUDIO_EVENTS` | `audio.transcript` |

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
# Docker (from contract root)
docker compose up -d --build clickhouse-consumer

# Local
make generate
cd clickhouse-consumer && go mod tidy
NATS_URL=nats://localhost:4222 CLICKHOUSE_HOST=localhost go run ./cmd/consumer

# Map unit tests (no NATS/CH required)
go test ./internal/map/ -count=1
```
