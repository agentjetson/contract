# AgentJetson — System Overview & Run Guide

This document is the single source of truth for how the AgentJetson edge CV + audio pipeline is structured and how to bring the full stack up. Treat it as the breadcrumb trail: follow the dependency order and the architecture will stay coherent as we expand.

**Primary vision backend:** [agentjetson/rf-detr](https://github.com/agentjetson/rf-detr) (RF-DETR via ONNX Runtime — TensorRT → CUDA → CPU) — *private*.

**Scene routing backends:** SigLIP 2 / DINOv3 (visual) + MoViNet (temporal) — see [scene-router](https://github.com/agentjetson/scene-router) and [temporal-classifier](https://github.com/agentjetson/temporal-classifier).

**This repo owns the contracts and the durable Go stack.**

| Surface | Location |
|---------|----------|
| `.proto` files (Buf) | `proto/` |
| ClickHouse DDL + seed | `seed/sql/` |
| NATS subject map | `domain/nats-subjects.yaml` |
| Scene taxonomy | `domain/taxonomy.yaml` |
| **ingest** (gRPC → NATS) | `ingest/` |
| **nats-publisher** | `nats-publisher/` |
| **aggregator** | `aggregator/` |
| **clickhouse-consumer** | `clickhouse-consumer/` |
| **query-service** (HTTP + MCP) | `query-service/` |
| **object-storage** (blob sink) | `object-storage/` |

Other repos consume; they do not keep a private copy of protos or DDL.

See [`CONSUMING.md`](CONSUMING.md), [`ARCHITECTURE.md`](ARCHITECTURE.md), [`seed/README.md`](seed/README.md), [`seed/WRITE_SPEC.md`](seed/WRITE_SPEC.md).

---

### Quick architecture

```
edge C++ (camera / scene / detect / specialists)
        │  IngestObject / IngestScene / IngestTranscript
        ▼
core (Go):  ingest → nats-publisher → JetStream
                      │
                      ├─► specialists (alpr, …) → cv.result.*
                      ├─► aggregator → cv.alert
                      └─► clickhouse-consumer → ClickHouse
                                                  │
                                                  ▼
                                          query-service (read-only)
```

### Stable contracts

| Contract           | Direction                               | Subject / RPC                           | Owner                               |
| ------------------ | --------------------------------------- | --------------------------------------- | ----------------------------------- |
| `FrameEnvelope`    | camera-connector → classifier/router    | (future gRPC / NATS)                    | camera-connector                    |
| `SceneResult`      | scene-router / temporal → ingest        | `cv.scene.<level1>` / `cv.scene.result` | **scene-router**                    |
| `ObjectEnvelope`   | classifier / preparator → ingest        | `cv.object.<class>` / `IngestObject`    | **stable**                          |
| `CapabilityResult` | specialists → aggregator                | `cv.result.<capability>`                | specialists                         |
| `Alert`            | aggregator → consumers                  | `cv.alert`                              | **core/aggregator**             |
| `Transcript`       | audio-client → ingest                   | `IngestTranscript` / `audio.transcript` | **voice/audio-client**              |
| **Query APIs**     | query-service → ClickHouse / NATS       | HTTP `/v1/query/*` + MCP tools          | **core/query-service**          |
| **Object storage** | edge producers → object-storage         | `PutObject` / HTTP `/v1/objects`        | **core/object-storage**         |

**ObjectEnvelope remains the cornerstone for detection.** Everything upstream of it can evolve. Everything downstream must not care how the envelope was produced.

**SceneResult is the upstream situation contract.** Answers “what kind of situation is this?” and carries Level-1 / Level-2 labels plus a suggested specialist list.

**query-service is a pure consumer.** It never publishes back into the CV pipeline.

**object-storage is a pure sink.** Stores blobs + correlation metadata; never publishes onto JetStream.

**audio-client is the pure STT recorder.** No LLM, no TTS.

**agent is the interactive front-end.** Calls query-service over HTTP for scene-aware answers.

---

## Repository map

| Repository | Role | Depends on |
| ---------- | ---- | ---------- |
| **[core](https://github.com/agentjetson/core)** (this repo) | public | protos, DDL, domain, **ingest, nats-publisher, aggregator, clickhouse-consumer, query-service, object-storage** | Docker Compose |
| [video](https://github.com/agentjetson/video) | C++ demo Alert `consumer`, `video_server`, `video_viewer` | core ClickHouse |
| [rf-detr](https://github.com/agentjetson/rf-detr) | Shared C++ RF-DETR ONNX library | OpenCV, ONNX Runtime |
| [camera-connector](https://github.com/agentjetson/camera-connector) | Thin multi-source capture (V4L2 / RTSP / file) | OpenCV |
| [camera-connector-onvif](https://github.com/agentjetson/camera-connector-onvif) | ONVIF discovery + stream resolve on top of camera-connector | libonvif, libcurl |
| [object-classifier](https://github.com/agentjetson/object-classifier) | Primary detect + track → ObjectEnvelope | **rf-detr**, core ingest |
| [crop-preparator](https://github.com/agentjetson/crop-preparator) | Multi-ROI crops (demo; production wiring TODO) | OpenCV |
| [scene-router](https://github.com/agentjetson/scene-router) | Hierarchical scene + specialist gating | ONNX (SigLIP 2 / DINOv3) |
| [temporal-classifier](https://github.com/agentjetson/temporal-classifier) | Temporal refine (MoViNet) | scene-router, ONNX |
| [alpr-consumer](https://github.com/agentjetson/alpr-consumer) | ALPR specialist | NATS, **rf-detr**, Fast-Plate-OCR |
| **[voice](https://github.com/agentjetson/voice)** | C++ speech (audio-client + agent) | core ingest + query-service, sherpa-onnx |

### Repo layout

| Directory / surface | Role |
| ------------------- | ---- |
| `proto/` | Canonical `.proto` files (Buf workspace) |
| `gen/go/` | buf-generated protos (make generate / Docker) |
| `domain/` | Scene taxonomy, NATS subject map |
| `seed/` | ClickHouse DDL + demo seed (`sql/001`–`004`) sole schema owner |
| `pkg/persistence` | Shared ClickHouse client + typed inserts (no DDL) + batch writers |
| `pkg/natsjs` | Shared JetStream helpers |
| `ingest/` | gRPC `:50052` → nats-publisher |
| `nats-publisher/` | JetStream publish surface `:50051` |
| `aggregator/` | Correlate objects + results → `cv.alert` durables agg-objects / agg-results |
| `clickhouse-consumer/` | JetStream pull → ClickHouse persistence |
| `query-service/` | Read-only Go HTTP `:8080` + MCP - SELECTs via persistence on query_* views |
| `object-storage/` | Blob sink — MinIO/S3 or filesystem; `:8081` / `:50055` uses persistence.InsertObjectMeta |
| `docker-compose.yml` | Full durable stack (NATS + CH + Go services + MinIO) |
| `Makefile` | `up` / `schema` / `seed` / `generate` |

### Voice layout (`agentjetson/voice`)

| Directory | Role |
| --------- | ---- |
| `audio-client/` | Edge STT live client → gRPC `:50054` + optional ingest |
| `agent/` | Interactive agent (KWS/VAD → STT → LLM → TTS); calls **core/query-service** |

Shared: `scripts/download_models.sh`, `models/` (gitignored).

**Model roles (CV)**

| Role | Repo | Model example |
| ---- | ---- | ------------- |
| Scene embedding / zero-shot router | scene-router | `models/siglip2-base-patch16-224.onnx` |
| Pure visual embedding (alt) | scene-router | `models/dinov3-vits16.onnx` |
| Temporal / action | temporal-classifier | `models/movinet_a0.onnx` |
| Primary detect | object-classifier via **rf-detr** | `models/rf-detr-nano.onnx` |
| Plate detect | alpr-consumer via **rf-detr** | `models/rfdetr-plate.onnx` |
| Plate OCR | alpr-consumer | `models/cct_s_v2_global.onnx` + yaml |

**Model roles (voice — under `voice/models/`)**

| Role | Component | Pack example |
| ---- | --------- | ------------ |
| Streaming STT | audio-client, agent | Zipformer or Nemotron 0.6B |
| VAD | agent | `silero_vad.onnx` |
| TTS | agent | `vits-piper-en_US-lessac-medium` |
| Hotword (KWS) | agent (optional) | `sherpa-onnx-kws-…` |

---

## Bring-up order (dependencies first)

### 0. Prerequisites

- CMake ≥ 3.20, C++17/20
- OpenCV 4.x / 5.x
- protobuf + gRPC
- **ONNX Runtime** (object-classifier, alpr-consumer, rf-detr, scene-router, temporal-classifier)
- (voice) **sherpa-onnx** + PortAudio — `SHERPA_ONNX_ROOT`
- Go 1.26+ + Buf CLI
- Docker Compose

```bash
export ONNXRUNTIME_ROOT=/path/to/onnxruntime
export SHERPA_ONNX_ROOT=$HOME/work/libs/sherpa-onnx
export LD_LIBRARY_PATH=$SHERPA_ONNX_ROOT/lib:$LD_LIBRARY_PATH
```

### 1. Contract stack (NATS + ClickHouse + Go services)

```bash
git clone https://github.com/agentjetson/core.git
cd core

make up          # full compose: nats, clickhouse, ingest, nats-publisher,
                 # aggregator, clickhouse-consumer, minio, object-storage, query-service
make schema      # idempotent DDL (if not applied by init scripts)
make seed        # demo rows (seed.cam-* / seed.mic-*)
```

Verify:

```bash
curl -s http://localhost:8222/healthz          # NATS monitor
curl -s http://localhost:8123/ping             # ClickHouse HTTP
curl -s http://localhost:8080/health | jq      # query-service
# ingest gRPC :50052, nats-publisher :50051, object-storage :8081
```

Or run individual Go services:

```bash
# query-service (DEMO_MODE needs no ClickHouse)
cd query-service && make generate && make tidy
export DEMO_MODE=true
go run ./cmd/server          # :8080

# object-storage
cd ../object-storage
export DEMO_MODE=true
go run ./cmd/server          # :8081 HTTP, :50055 gRPC
```

MCP / stdio:

```bash
go run ./cmd/server --stdio
# tools: query_recent_plates, query_objects, query_scenes,
#        query_detections, query_transcripts, health
```

### 2. alpr-consumer (specialist)

```bash
git clone https://github.com/agentjetson/alpr-consumer.git
cd alpr-consumer
# place models: rfdetr-plate.onnx, cct_s_v2_global.onnx, plate config yaml
cmake -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build -j$(nproc)

NATS_URL=nats://localhost:4222 \
ORT_DEVICE=gpu \
PLATE_DETECTOR_MODEL=models/rfdetr-plate.onnx \
PLATE_OCR_MODEL=models/cct_s_v2_global.onnx \
./build/alpr_consumer
```

Durable consumer `alpr-worker` on `CV_EVENTS` filter `cv.object.*` → publishes `cv.result.alpr`.

### 3. CV edge path

camera-connector → scene-router → temporal-classifier → object-classifier → (optional) crop-preparator.

Single-box demos prefer `--in-process` capture (no separate camera-connector process). See each repo’s README.

```bash
# Example: object-classifier
INGEST_ADDR=localhost:50052 ORT_DEVICE=gpu \
  ./build/object_classifier --in-process sample.mp4 models/rf-detr-nano.onnx
```

### 4. Video path

```bash
git clone https://github.com/agentjetson/video.git
cd video
docker compose up -d video_server video_viewer
```

### 5. Voice path

```bash
git clone https://github.com/agentjetson/voice.git
cd voice
./scripts/download_models.sh

# audio-client
cd audio-client
cmake -B build -DCMAKE_BUILD_TYPE=Release -DSHERPA_ONNX_ROOT=$SHERPA_ONNX_ROOT
cmake --build build -j$(nproc) --target audio_client
LIVE_ADDR=0.0.0.0:50054 INGEST_ADDR=localhost:50052 SOURCE=mic \
  SHERPA_MODEL_TYPE=zipformer \
  SHERPA_MODEL_DIR=../models/sherpa-onnx-streaming-zipformer-en-2023-06-26 \
  ./build/audio_client

# agent
cd ../agent
cmake -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build -j$(nproc)
ollama pull tinyllama && ollama serve &
./build/voice_agent --models-dir ../models --query-url http://127.0.0.1:8080
```

---

## ClickHouse schema (this repo only)

One schema. Applied by `make schema` / compose init scripts. **No other repo CREATE TABLEs.**

```bash
make up && make schema && make seed
```

| Subject | Proto | Table | Writer |
| ------- | ----- | ----- | ------ |
| `cv.object.>` | `detection.v1.ObjectEnvelope` | `cv_objects` | **core/clickhouse-consumer** (in progress) |
| `cv.result.>` | `detection.v1.CapabilityResult` | `cv_results` | **core/clickhouse-consumer** (in progress) |
| `cv.scene.>` | `scene.v1.SceneResult` | `cv_scenes` | **core/clickhouse-consumer** (in progress) |
| `cv.alert` | `detection.v1.Alert` | `cv_detections` | **core/clickhouse-consumer** |
| `audio.transcript` | `audio.v1.Transcript` | `audio_transcripts` | **core/clickhouse-consumer** |

`query_*` views (`seed/sql/003_views.sql`) alias columns for query-service. Until full writers land, use `make seed` or `DEMO_MODE=true`.

---

## Design principles

1. **Capture is not classification.** camera-connector owns sources; object-classifier owns detection; **scene-router owns situation**.
2. **ObjectEnvelope is the stable detection contract.**
3. **SceneResult is the stable situation contract.** Does not replace ObjectEnvelope.
4. **Specialists are pure consumers** of ObjectEnvelopes.
5. **query-service is a pure consumer.** Never publishes into the CV pipeline.
6. **object-storage is a pure sink.** Never publishes onto JetStream.
7. **audio-client is the pure recorder.** No LLM, no TTS.
8. **agent is the interactive front-end.** Calls query-service for answers.
9. **One detection engine** — agentjetson/rf-detr; swap weights, not frameworks.
10. **Scene ≠ detection.** Keep situation models separate from presence models.
11. **Hierarchical taxonomy** (L1→L2) — attach specialists to the tree.
12. **Zero-shot first, fine-tune later.**
13. **Temporal only when needed.**
14. **Crop quality is separable** (crop-preparator).
15. **Edge-first.** Heavy inference near the camera; contract is correlation + durable bus + query.
16. **Composable.** New cameras / models / capabilities / query tools / blob sinks without rewriting the bus.
17. **Contracts + durable Go services live here.** Other repos consume; they do not fork copies.
18. **Shared speech models** under `voice/models/` with shared `SHERPA_*` env names.

---

## Known gaps & refactor targets

| Item | Status |
|------|--------|
| clickhouse-consumer writers for `cv_objects` / `cv_results` / `cv_scenes` | In progress — see TODO.md |
| `pkg/persistence` wired into object-storage + query-service | TODO |
| Taxonomy single-source (`domain/taxonomy.yaml` vs scene-router prompts) | Drift — align |
| docker-compose MinIO port 9000 vs ClickHouse 9000 | Clash — remap MinIO |
| External `core_net` assumption in compose | Fragile — make internal or document |
| Service name `voice-query-service` vs `query-service` | Rename for consistency |
| crop-preparator production NATS/gRPC path | Stub only |
| core residual README / CMake (`edge_proto` without local proto/) | Needs cleanup |
| rf-detr + camera-connector still private | Consider public if demos need them |

---

## Future expansion

| Area | Next step |
|------|-----------|
| Camera ecosystem | ONVIF (started in camera-connector-onvif), proprietary SDKs, NVMM/DMA-BUF |
| Frame hand-off | gRPC streaming / NATS `FrameEnvelope` |
| Scene routing | Production SigLIP/DINOv3 sessions + gating into object-classifier |
| Temporal path | Full MoViNet L2 map; side-channel from scene-router |
| Specialists | vehicle_attr, face, person_attr, damage, … |
| Query surface | More MCP tools; live NATS peek filters |
| Object storage | Full gRPC surface; retention policies |
