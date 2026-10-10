# ingest (Go)

Edge → core gRPC ingress **and** JetStream publisher — combined binary.

On a single Orin the former `ingest` → `nats-publisher` gRPC hop was pure
overhead. This service accepts the same `ingest.v1.IngestService` RPCs, owns
the NATS connection, and publishes directly. It also registers
`nats.v1.NatsPublisherService` on the same gRPC server so specialists that
dial the publisher surface keep working.

| RPC              | Publishes to (default subject) |
|------------------|--------------------------------|
| IngestAlert      | `cv.alert`                     |
| IngestTranscript | `audio.transcript`             |
| IngestObject     | `cv.object.<class_name>`       |
| IngestResult     | `cv.result.<capability>`       |
| IngestScene      | `cv.scene.<level1>`            |

## Env

| Variable           | Default                 | Role                                      |
|--------------------|-------------------------|-------------------------------------------|
| `GRPC_ADDR`        | `0.0.0.0:50052`         | Listen for IngestService + NatsPublisherService |
| `NATS_URL`         | `nats://localhost:4222` | JetStream                                 |
| `NATS_CLIENT_NAME` | `ingest`                | NATS client name                          |

`PUBLISHER_ADDR` is no longer used.

## Build & run

```bash
# from core root
make generate
cd ingest
go mod tidy
go run ./cmd/server

# env
GRPC_ADDR=0.0.0.0:50052
NATS_URL=nats://localhost:4222
```

Docker (monorepo context):

```bash
docker build -f ingest/Dockerfile -t ingest-server .
```
