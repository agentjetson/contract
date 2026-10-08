# ingest (Go)

Edge → core gRPC ingress — Go port of `core/src/ingest`.

Thin pass-through onto `nats.v1.NatsPublisherService`. Does **not** own a NATS
connection; that lives in `nats-publisher`.

| RPC              | Forwards to              | Default subject (set by publisher) |
|------------------|--------------------------|------------------------------------|
| IngestAlert      | PublishAlert             | `cv.alert`                         |
| IngestTranscript | PublishTranscript        | `audio.transcript`                 |
| IngestObject     | PublishObject            | `cv.object.<class_name>`           |
| IngestResult     | PublishResult            | `cv.result.<capability>`           |
| IngestScene      | PublishScene             | `cv.scene.<level1>`                |

`IngestScene` is implemented here (the C++ binary did not yet).

## Env

| Variable         | Default              | Role                                      |
|------------------|----------------------|-------------------------------------------|
| `GRPC_ADDR`      | `0.0.0.0:50052`      | Listen address for IngestService          |
| `PUBLISHER_ADDR` | `localhost:50051`    | Dial address for NatsPublisherService     |

## Build & run

```bash
# from contract root
make generate          # produces gen/go (required)
cd ingest
go mod tidy
go run ./cmd/server

# env
GRPC_ADDR=0.0.0.0:50052
PUBLISHER_ADDR=localhost:50051
```

Docker (monorepo context):

```bash
docker build -f ingest/Dockerfile -t ingest-server .
```

## Relation to core/

After this lands:

1. Remove C++ `ingest_server` binary / CMake target from `agentjetson/core`.
2. Point core `docker-compose` `ingest` service at this image (or run it from
   contract compose).
3. Edge clients (`object-classifier`, `scene-router`, `audio-client`, specialists)
   keep calling the same gRPC surface on `:50052`.
