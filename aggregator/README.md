# aggregator (Go)

Correlates `ObjectEnvelope` + `CapabilityResult` by `(frame_id, track_id)`,
applies the watchlist, and publishes `detection.v1.Alert` to `cv.alert`.

Pure NATS — **no ClickHouse writes**. Persistence is `clickhouse-consumer`
(`cv.alert` → `cv_detections`).

## Behaviour

| Input subject | Action |
|---------------|--------|
| `cv.object.>` | Store / update pending track; emit immediately if ALPR already present |
| `cv.result.>` (capability=`alpr`) | Attach OCR + plate box; emit if object already present |
| timeout ≈ 800 ms | Emit pending objects that never got ALPR (persons, etc.) |
| prune ≈ 5 s | Drop stale pending entries |

Watchlist defaults: `person@0.55`, `car@0.50`, `truck@0.50`, sample plate `AF29KX@0.60`.
Override with `WATCHLIST=person:0.6,car:0.5,ABC123:0.7`.

## Env

| Variable | Default |
|----------|---------|
| `NATS_URL` | `nats://localhost:4222` |
| `STREAM_EVENTS` | `CV_EVENTS` |
| `STREAM_ALERTS` | `CV_ALERTS` |
| `FETCH_BATCH` | `8` |
| `FETCH_TIMEOUT_MS` | `200` |
| `EMIT_TIMEOUT_MS` | `800` |
| `PRUNE_TIMEOUT_MS` | `5000` |
| `PRUNE_EVERY_MS` | `1000` |
| `WATCHLIST` | (built-in defaults) |

## JetStream

| Durable | Stream | Filter |
|---------|--------|--------|
| `agg-objects` | `CV_EVENTS` | `cv.object.>` |
| `agg-results` | `CV_EVENTS` | `cv.result.>` |

Streams are created by **nats-publisher** (`EnsureAllStreams` from
`domain/nats-subjects.yaml`). Aggregator waits for `CV_EVENTS` (required)
and warns if `CV_ALERTS` is late.

## Run

```bash
# Docker (from contract root)
docker compose up -d --build aggregator

# Local (needs generated stubs)
make generate
cd aggregator && go mod tidy
NATS_URL=nats://localhost:4222 go run ./cmd/aggregator

# Tests (need gen/go)
go test ./internal/correlate/ -count=1
```

## Design notes

- Durable names match the former C++ aggregator for zero-downtime swap.
- Publish subject is always `cv.alert` (proto `detection.v1.Alert`).
- Core residual stack no longer ships an aggregator binary.
