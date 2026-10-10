# nats-publisher (Go) — optional standalone

JetStream publish gRPC front-end. **On single-Orin deployments the combined
`ingest` binary owns NATS and registers `NatsPublisherService` on the same
port** (`:50052`), so this process is not started by default compose.

Keep this tree when you need independent scaling (multi-node / separate
publish plane). Protos and subjects are unchanged.

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

Shared helpers live in `pkg/natsjs`.

## Build & run (standalone)

```bash
make generate
cd nats-publisher
go mod tidy
go run ./cmd/publisher

NATS_URL=nats://localhost:4222
GRPC_ADDR=0.0.0.0:50051
```

Docker:

```bash
docker build -f nats-publisher/Dockerfile -t nats-publisher .
```
