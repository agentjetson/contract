# AgentJetson Architecture

```mermaid
flowchart TB
subgraph Edge Device
CAM[Cameras / NVRs / Bodycams / Dashcams<br/>ONVIF · RTSP · V4L2 · USB]
CC[camera-connector<br/>thin multi-source capture]
SR[scene-router<br/>SigLIP 2 / DINOv3<br/>hierarchical L1→L2]
TC[temporal-classifier<br/>MoViNet<br/>ambiguous temporal states]
OC[object-classifier<br/>primary detect + track<br/>rf-detr · ConsumerPipeline]
CP[crop-preparator<br/>OpenCV multi-ROI crops]
end

subgraph Contract["agentjetson/core (this repo)"]
ING[ingest<br/>gRPC :50052 → NATS publisher]
NP[nats-publisher<br/>JetStream :50051]
AGG[aggregator<br/>correlate + watchlist]
CHC[clickhouse-consumer<br/>JetStream → CH]
CH[(ClickHouse)]
VQS[query-service<br/>Go · HTTP :8080 · MCP]
OBS[object-storage<br/>Go · HTTP :8081 · gRPC :50055]
NATS[NATS JetStream]
end

subgraph Specialists
ALPR[alpr-consumer<br/>rf-detr plate + OCR]
end

subgraph Shared Engine
RFD[agentjetson/rf-detr<br/>ONNX RF-DETR library]
end

subgraph Voice["agentjetson/voice (C++ speech)"]
AC[audio-client<br/>sherpa-onnx STT · live gRPC :50054]
VA[agent<br/>KWS/VAD → STT → LLM → TTS]
LLM[Local LLM<br/>Ollama / llama.cpp]
end

subgraph Core Residual["agentjetson/core (residual)"]
CONS[consumer<br/>demo Alert stdout]
VS[video_server / video_viewer]
end

CAM -->|frames| CC
CC -->|FrameQueue / FrameEnvelope| SR
SR -->|SceneResult<br/>cv.scene.*| ING
SR -.->|temporal_requested| TC
TC -->|refined SceneResult| SR
SR -->|gating / specialist list| OC
CC -->|FrameQueue| OC
OC -->|ObjectEnvelope<br/>cv.object.*| ING
OC -.->|optional| CP
CP -->|enriched crops / ObjectEnvelope| ING

ING --> NP
NP --> NATS
NATS -->|cv.object.*| ALPR
NATS -->|cv.object.* + cv.result.* + cv.scene.*| AGG
ALPR -->|CapabilityResult<br/>cv.result.alpr| NP
AGG -->|Alert<br/>cv.alert| NATS
NATS --> CHC
NATS --> CONS
CHC --> CH

RFD -.->|primary model| OC
RFD -.->|plate model| ALPR

AC -->|IngestTranscript / audio.transcript| ING
AC -.->|optional shared STT| VA
AC -.->|optional PutObject| OBS
VA -->|transcript| LLM
VA -->|POST /v1/query/* libcurl| VQS
LLM -.->|future tool-calls| VQS
VQS -->|read-only queries| CH
VQS -.->|optional live peek| NATS
OBS -->|object_meta| CH
OBS -->|blobs| MinIO[(MinIO / S3)]
VS -->|reads cv_detections| CH

style CC fill:#1a365d,stroke:#63b3ed,color:#fff
style SR fill:#744210,stroke:#f6e05e,color:#fff
style TC fill:#744210,stroke:#f6e05e,color:#fff
style OC fill:#1a365d,stroke:#63b3ed,color:#fff
style CP fill:#2c5282,stroke:#90cdf4,color:#fff
style ING fill:#276749,stroke:#68d391,color:#fff
style AGG fill:#276749,stroke:#68d391,color:#fff
style ALPR fill:#744210,stroke:#f6e05e,color:#fff
style RFD fill:#553c9a,stroke:#b794f4,color:#fff
style AC fill:#2b6cb0,stroke:#90cdf4,color:#fff
style VA fill:#2b6cb0,stroke:#90cdf4,color:#fff
style VQS fill:#2d3748,stroke:#a0aec0,color:#fff
style OBS fill:#2d3748,stroke:#a0aec0,color:#fff
style CH fill:#2d3748,stroke:#a0aec0,color:#fff
style CONS fill:#718096,stroke:#a0aec0,color:#fff
```

## Subject hierarchy (JetStream)

Canonical map: [`domain/nats-subjects.yaml`](domain/nats-subjects.yaml).

| Stream | Subjects | Payload |
|--------|----------|---------|
| `CV_EVENTS` | `cv.object.>`, `cv.result.>`, `cv.scene.>` | ObjectEnvelope / CapabilityResult / SceneResult |
| `CV_ALERTS` | `cv.alert` | Alert |
| `AUDIO_EVENTS` | `audio.transcript` | Transcript |

| Subject pattern | Producer | Consumer(s) | Table |
|-----------------|----------|-------------|-------|
| `cv.object.<class>` | object-classifier / crop-preparator | alpr-consumer, aggregator, clickhouse-consumer | `cv_objects` |
| `cv.result.<capability>` | specialists (alpr, …) | aggregator, clickhouse-consumer | `cv_results` |
| `cv.scene.<level1>` / `cv.scene.result` | scene-router, temporal-classifier | aggregator, clickhouse-consumer | `cv_scenes` |
| `cv.alert` | aggregator | consumer (demo), clickhouse-consumer, video path | `cv_detections` |
| `audio.transcript` | audio-client | clickhouse-consumer | `audio_transcripts` |

## Flow

```
Cameras → object-classifier (or scene-router → object-classifier)
            │ detect + track + crop JPEG  /  SceneResult
            ▼
        IngestObject / IngestScene (gRPC :50052)
            ▼
        nats-publisher → JetStream
            │
            ├──────────────────► alpr-consumer
            │                      │ plate OCR on crop
            │                      ▼
            │                   cv.result.alpr
            │                      │
            └──────────────────► aggregator
                                   │ correlate by (frame_id, track_id)
                                   │ apply watchlist / policy
                                   ▼
                                cv.alert
                                   │
                                   ├─► consumer (demo stdout — core)
                                   └─► clickhouse-consumer → ClickHouse
```

## Components

### Edge (C++)
- **camera-connector** (private) / **camera-connector-onvif** — capture only.
- **scene-router** — hierarchical L1→L2 situation; specialist gating hints.
- **temporal-classifier** — MoViNet refine when `temporal_requested`.
- **object-classifier** — RF-DETR detect + track → `ObjectEnvelope`.
- **crop-preparator** — multi-ROI crops (production NATS path still TODO).
- **alpr-consumer** — pure specialist: `ObjectEnvelope` in → `CapabilityResult` out.

### Contract (this repo — Go)
- **ingest** — gRPC `:50052`; fans out to nats-publisher.
- **nats-publisher** — JetStream publish surface `:50051`.
- **aggregator** — correlates objects + capability results; emits `cv.alert`.
- **clickhouse-consumer** — durable JetStream → ClickHouse writers.
- **query-service** — read-only HTTP `:8080` + MCP; never publishes.
- **object-storage** — blob sink HTTP `:8081` / gRPC `:50055`; metadata to CH.

### Core residual (C++)
- **consumer** — demo Alert stdout consumer.
- **video_server / video_viewer** — annotated video path over ClickHouse `cv_detections`.

### Voice (C++)
- **audio-client** — pure STT recorder + live gRPC + optional ingest.
- **agent** — interactive front-end; calls query-service for scene-aware answers.

## Adding a new specialist (e.g. vehicle colour)

1. Copy `alpr-consumer` pattern → new binary.
2. Filter / capability string / inference.
3. Publish to `cv.result.<capability>`.
4. Extend aggregator to merge attributes into Alert (or keep as side-channel).
5. Ensure clickhouse-consumer maps the capability into `cv_results`.

## Known gaps (see also TODO.md)

- clickhouse-consumer must fully write `cv_objects` / `cv_results` / `cv_scenes` (partial today).
- `pkg/persistence` integration into object-storage + query-service unfinished.
- Taxonomy: `domain/taxonomy.yaml` vs scene-router README prompts — single-source required.
- docker-compose: MinIO vs ClickHouse port 9000 clash; external `core_net` assumption.
