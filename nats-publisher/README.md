# nats-publisher (Go)

JetStream publish gRPC front-end — Go port of `core/src/nats_publisher`.

Owns the NATS / JetStream connection and exposes `nats.v1.NatsPublisherService`:

| RPC               | Default subject              | Payload                        |
|-------------------|------------------------------|--------------------------------|
| PublishAlert      | `cv.alert`                   | `detection.v1.Alert`           |
| PublishTranscript | `audio.transcript`           | `audio.v1.Transcript`          |
| PublishObject     | `cv.object.<class_name>`     | `detection.v1.ObjectEnvelope`  |
| PublishResult     | `cv.result.<capability>`     | `detection.v1.CapabilityResult`|
| PublishScene      | `cv.scene.<level1>`          | `scene.v1.SceneResult`         |

## Streams (from `domain/nats-subjects.yaml`)

On startup the service ensures:

- **CV_EVENTS** → `cv.object.>`, `cv.result.>`, `cv.scene.>` (1h retention)
- **CV_ALERTS** → `cv.alert`
- **AUDIO_EVENTS** → `audio.transcript` (non-fatal if already owned)

Shared helpers live in `pkg/natsjs` so `clickhouse-consumer` (and aggregator) can reuse Connect / EnsureStream.

## Build & run

```bash
# from contract root
make generate          # produces gen/go (required)
cd nats-publisher
go mod tidy
go run ./cmd/publisher

# env
NATS_URL=nats://localhost:4222
GRPC_ADDR=0.0.0.0:50051
```

Docker (monorepo context):

```bash
docker build -f nats-publisher/Dockerfile -t nats-publisher .
```

## Relation to core/

After this lands:

1. Remove C++ `nats_publisher` binary / CMake target from `agentjetson/core`.
2. Point core `docker-compose` `nats-publisher` service at this image (or run it from contract compose).
3. Edge clients (`ingest_server`, specialists) keep calling the same gRPC surface.
