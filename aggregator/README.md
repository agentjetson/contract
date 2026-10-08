# aggregator (Go)

Correlates `ObjectEnvelope` + `CapabilityResult` by `(frame_id, track_id)`,
applies the watchlist, and publishes `detection.v1.Alert` to `cv.alert`.

Go port of `core/src/aggregator/main.cpp`. Replaces the C++ binary once
cut over in compose.

## Behaviour (parity with C++)

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

## Run

```bash
# from contract root (after NATS + nats_publisher are up)
cd aggregator
go mod tidy
export NATS_URL=nats://localhost:4222
go run ./cmd/aggregator
```

Requires generated stubs (`make generate` at contract root) so
`github.com/agentjetson/contract/gen/go/detection/v1` resolves.

## Core cutover

1. Build/run this service instead of `core`’s `/app/aggregator`.
2. Point compose `command` / image at the Go binary.
3. Remove `src/aggregator` and its CMake target from `core/`.
4. Keep `clickhouse-consumer` as the durable sink for `cv.alert` → `cv_detections`.

## Design notes

- **No ClickHouse writes** — pure NATS correlation + publish. Persistence
  stays in `pkg/persistence` via `clickhouse-consumer`.
- **Durable pull consumers** `agg-objects` / `agg-results` on `CV_EVENTS`
  (same names as the C++ version for in-place swap).
- **Publish** uses JetStream `Publish("cv.alert", …)`; stream ownership
  remains with `nats_publisher` (`CV_ALERTS`).
