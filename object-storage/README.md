# object-storage

**Durable blob store for AgentJetson audio transcripts and camera recordings.**

Stores binary artefacts (audio segments, continuous / event video, frame
snapshots, crops, annotated clips) in an S3-compatible backend (MinIO or AWS S3)
or a local filesystem for demos.  Every successful put writes a correlation row
that query-service and ClickHouse can join back to `cv.object.*`,
`cv.scene.*`, `audio.transcript`, and `cv.alert`.

This is a **pure consumer / sink** relative to the CV + audio pipeline: nothing
publishes back onto NATS JetStream.  Upstream producers (audio-client,
camera-connector / object-classifier, future recording agents) call via gRPC
and receive an `object_id` + `storage_key` they can
embed in their existing envelopes or ClickHouse rows.

---

## Why Go (not C++)

| Concern | C++ (edge path) | Go (this service) |
| --- | --- | --- |
| Role | ONNX inference, low-latency capture | Durable blob I/O + metadata |
| Existing pattern | object-classifier, alpr-consumer, rf-detr | **query-service** (same monorepo family) |
| Libraries | OpenCV, ORT, gRPC-C++ | minio-go, aws-sdk-go, native gRPC |
| Ops | Jetson builds, CUDA | Docker / K8s, trivial cross-compile |
| Correlation | Not needed at edge | ClickHouse writer, ListObjects by time/source |

**Recommendation: keep the gRPC object-storage service in Go**, matching
`voice/query-service`.  The edge producers stay C++; they only need a thin
gRPC client (or the HTTP demo surface) to ship blobs.  Contracts live in
`proto/storage/v1` and will move into the shared Buf package alongside the
other `agentjetson/contract` protos.

---

## Architecture fit

```
audio-client ──IngestTranscript──► core (NATS / CH)
       │
       └── optional PutObject(AUDIO_TRANSCRIPT) ──► object-storage ──► MinIO/S3
                                                      │
camera-connector / recording agent                    ├── object_meta → ClickHouse
       │                                              │
       └── PutObject(CAMERA_RECORDING / FRAME) ───────┘
                                                      │
query-service ◄── ListObjects / GetPresignedURL ──────┘
       │
       └── joins object_id / storage_key with cv.* + audio.transcript
```

### Contracts to extend (`agentjetson/contract`)

| Existing proto | Extension needed |
| --- | --- |
| `audio/v1/transcript.proto` | Optional `string object_id = 12` / `string storage_key = 13` so a transcript row can point at the raw audio blob |
| `detection/v1/detection.proto` (`ObjectEnvelope`, `Alert`) | Optional `string recording_object_id` / `string snapshot_object_id` for evidence packaging |
| `capture/v1/frame.proto` | Optional `payload_ref` already exists — prefer object-storage keys over ad-hoc shm paths for durable refs |
| **New** `storage/v1/object_storage.proto` | This repo (canonical definition; copy or submodule into `contract` when stabilised) |

No change is required to the stable `ObjectEnvelope` / `SceneResult` /
`CapabilityResult` publish path.  Object-storage is additive.

---

## Quick start (DEMO_MODE — no MinIO / ClickHouse)

```bash
git clone https://github.com/agentjetson/object-storage && cd object-storage

# filesystem backend under ./data
export DEMO_MODE=true
go run ./cmd/server
# HTTP :8081   gRPC port reserved :50055
```

```bash
# Put a fake audio transcript blob
curl -s -X POST 'http://localhost:8081/v1/objects?kind=audio_transcript&source=mic-1' \
  -H 'Content-Type: audio/wav' \
  --data-binary @sample.wav | jq

# List
curl -s 'http://localhost:8081/v1/objects?source=mic-1' | jq

# Get meta
curl -s 'http://localhost:8081/v1/objects/<object_id>?meta=1' | jq

# Download
curl -s -o /tmp/out.wav 'http://localhost:8081/v1/objects/<object_id>'
```

---

## Production (MinIO + optional ClickHouse)

```bash
# 1. Start MinIO (or use the compose file below)
docker compose up -d minio minio-init

# 2. Run the service
export DEMO_MODE=false
export STORAGE_BACKEND=minio
export S3_ENDPOINT=localhost:9000
export S3_ACCESS_KEY=minioadmin
export S3_SECRET_KEY=minioadmin
export S3_BUCKET=agentjetson
export S3_USE_SSL=false
export CLICKHOUSE_ENABLED=true
export CLICKHOUSE_HOST=localhost
export CLICKHOUSE_PORT=9000
export CLICKHOUSE_USER=default
export CLICKHOUSE_PASSWORD=pass
export CLICKHOUSE_DB=default
go run ./cmd/server
```

---

## Docker Compose

```bash
docker compose up --build
# object-storage :8081 (HTTP) + :50055 (gRPC reserved)
# minio          :9000 (API)  + :9001 (console)
```

Overlay onto the core network:

```bash
docker compose -f docker-compose.yml -f docker-compose.core.yml up -d
```

---

## Environment

| Variable | Default | Notes |
| --- | --- | --- |
| `DEMO_MODE` | `true` | Forces filesystem backend, disables CH |
| `STORAGE_BACKEND` | `minio` | `minio` \| `s3` \| `filesystem` |
| `S3_ENDPOINT` | `localhost:9000` | Host:port, no scheme |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | `minioadmin` | |
| `S3_BUCKET` | `agentjetson` | Created on start if missing |
| `S3_USE_SSL` | `false` | |
| `S3_REGION` | `us-east-1` | |
| `S3_FORCE_PATH_STYLE` | `true` | Required for MinIO |
| `FS_ROOT` | `./data` | Used when backend=filesystem |
| `GRPC_ADDR` | `0.0.0.0:50055` | Reserved for generated gRPC server |
| `HTTP_ADDR` | `0.0.0.0:8081` | Demo / integration HTTP surface |
| `CLICKHOUSE_ENABLED` | `false` | When true (and not DEMO_MODE), metadata uses `pkg/persistence` → `object_meta` |
| `CLICKHOUSE_HOST` | `localhost` | Native protocol host |
| `CLICKHOUSE_PORT` | `9000` | Native protocol port |
| `CLICKHOUSE_USER` / `CLICKHOUSE_PASSWORD` | `default` / `pass` | |
| `CLICKHOUSE_DB` | `default` | Also accepts `CLICKHOUSE_DATABASE` |
| `CLICKHOUSE_DSN` | (optional) | Docs/compose alias; discrete vars above are what Open() uses |

After every successful Put, the service calls `InsertObjectMeta`. Get / List / MarkDeleted hit the same `object_meta` table (schema from `make schema`). DEMO_MODE keeps `MemoryStore` only.

---

## Object key layout

```
{kind}/{source}/{YYYY}/{MM}/{DD}/{object_id}.{ext}

examples:
  audio_transcript/mic-1/2026/10/07/a1b2c3d4-….wav
  camera_recording/cam-front/2026/10/07/e5f6….mp4
  frame_snapshot/cam-front/2026/10/07/789a….jpg
  crop/cam-front/2026/10/07/bcde….jpg
```

---

## Proto & code generation

```
proto/
  common/v1/error.proto          # vendored from contract
  storage/v1/object_storage.proto
```

When the shared Buf workspace lands:

```bash
buf generate   # produces gen/storage/v1 + gen/common/v1
# then replace the hand-written types in internal/server with the generated stubs
```

Until then the HTTP surface + hand-written service types are fully usable for
integration tests and the first demo path.

---

## Integration checklist

1. **audio-client** (optional): after a final transcript, also `PutObject(AUDIO_TRANSCRIPT)` with the raw audio window; store returned `object_id` on the `Transcript` (once the field is added in contract).
2. **camera / recording path**: event or continuous recorder calls `PutObject(CAMERA_RECORDING)` and attaches `object_id` to the correlating `Alert` or a new evidence table.
3. **query-service**: add MCP / HTTP tools `query_recordings`, `get_presigned_url` that read `object_meta` (and optionally live ListObjects).
4. **core clickhouse_consumer**: no change required until you want NATS-driven archival; the preferred path is direct gRPC/HTTP from the edge producer.

---

## Design principles (aligned with the system overview)

1. **Capture is not classification; storage is not correlation.**  This service only stores and indexes blobs.
2. **ObjectEnvelope / SceneResult / Transcript stay the stable event contracts.**  Object-storage is an additive evidence plane.
3. **query-service remains a pure consumer.**  It may call GetPresignedURL / ListObjects; it never writes blobs.
4. **Edge-first producers, central durable store.**  Heavy inference stays on-device; blobs land in object storage as soon as the network allows.
5. **Contracts in `.proto` files**, generated with Buf, shared via `agentjetson/contract`.

---

## Ports

| Surface | Port |
| --- | --- |
| HTTP (demo / integration) | **8081** |
| gRPC (generated) | **50055** |
| MinIO API | 9000 |
| MinIO console | 9001 |

---

*Local trail marker.  The architecture map lives in `agentjetson/contract` (System Overview & Run Guide).  Keep both in sync when the evidence plane evolves.*
