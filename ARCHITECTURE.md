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
ING[ingest<br/>gRPC :50052 · owns NATS<br/>IngestService + NatsPublisherService]
SG[scene-gate<br/>profile + audio intent → SceneResult]
AGG[aggregator<br/>correlate + watchlist]
CHC[clickhouse-consumer<br/>JetStream → CH]
CH[(ClickHouse)]
VQS[voice-query-service<br/>Go · HTTP :8080 · MCP]
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

ING --> NATS
NATS -->|cv.object.*| ALPR
NATS -->|cv.object.* + cv.result.* + cv.scene.*| AGG
ALPR -->|CapabilityResult<br/>cv.result.alpr| ING
AGG -->|Alert<br/>cv.alert| NATS
NATS --> CHC
NATS --> CONS
CHC --> CH

RFD -.->|primary model| OC
RFD -.->|plate model| ALPR

AC -->|IngestTranscript / audio.transcript| ING
NATS -->|audio.transcript| SG
SG -->|IngestScene short-circuit| ING
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
style SG fill:#276749,stroke:#68d391,color:#fff
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
| `cv.scene.<level1>` / `cv.scene.result` | scene-router, temporal-classifier, **scene-gate** | aggregator, clickhouse-consumer | `cv_scenes` |
| `cv.alert` | aggregator | consumer (demo), clickhouse-consumer, video path | `cv_detections` |
| `audio.transcript` | audio-client | clickhouse-consumer, **scene-gate** | `audio_transcripts` |

## Flow

```
Cameras → object-classifier (or scene-router → object-classifier)
            │ detect + track + crop JPEG  /  SceneResult
            ▼
        IngestObject / IngestScene (gRPC :50052)
            ▼
        ingest (owns NATS) → JetStream
            │
            ├──────────────────► alpr-consumer
            │                      │ plate OCR on crop
            │                      ▼
            │                   cv.result.alpr (via IngestResult or NatsPublisherService)
            │                      │
            └──────────────────► aggregator
                                   │ correlate by (frame_id, track_id)
                                   │ apply watchlist / policy
                                   ▼
                                cv.alert
                                   │
                                   ├─► consumer (demo stdout — core)
                                   └─► clickhouse-consumer → ClickHouse

Audio path (body-cam / POV):
  audio-client → audio.transcript → scene-gate
       │                                │
       │                     EmitNow ───┼──► IngestScene (backend=audio|profile)
       │                     ForwardVisual → edge scene-router still runs
```

## Components

### Edge (C++)
- **camera-connector** (private) / **camera-connector-onvif** — capture only.
- **scene-router** — hierarchical L1→L2 situation; specialist gating hints (runs when gate forwards).
- **temporal-classifier** — MoViNet refine when `temporal_requested`.
- **object-classifier** — RF-DETR detect + track → `ObjectEnvelope`.
- **crop-preparator** — multi-ROI crops (production NATS path still TODO).
- **alpr-consumer** — pure specialist: `ObjectEnvelope` in → `CapabilityResult` out.

### Contract (this repo — Go)
- **ingest** — gRPC `:50052`; owns NATS; registers both `IngestService` and `NatsPublisherService`.
- **scene-gate** — per-camera policy: fixed-role profile short-circuit + audio intent → `SceneResult`; otherwise forward to visual path. Config: `domain/camera_profiles.yaml` + `domain/taxonomy.yaml`.
- **aggregator** — correlates objects + capability results; emits `cv.alert`.
- **clickhouse-consumer** — durable JetStream → ClickHouse writers.
- **voice-query-service** — read-only HTTP `:8080` + MCP; never publishes.
- **object-storage** — blob sink HTTP `:8081` / gRPC `:50055`; metadata to CH.

### Core residual (C++)
- **consumer** — demo Alert stdout consumer.
- **video_server / video_viewer** — annotated video path over ClickHouse `cv_detections`.

### Voice (C++)
- **audio-client** — pure STT recorder + live gRPC + optional ingest.
- **agent** — interactive front-end; calls voice-query-service for scene-aware answers.

## Adding a new specialist (e.g. vehicle colour)

1. Copy `alpr-consumer` pattern → new binary.
2. Filter / capability string / inference.
3. Publish to `cv.result.<capability>` (via IngestResult or NatsPublisherService on `:50052`).
4. Extend aggregator to merge attributes into Alert (or keep as side-channel).
5. Ensure clickhouse-consumer maps the capability into `cv_results`.

## Known gaps (see also TODO.md)

- clickhouse-consumer must fully write `cv_objects` / `cv_results` / `cv_scenes` (partial today).
- `pkg/persistence` integration into object-storage + voice-query-service unfinished.
- Taxonomy: `domain/taxonomy.yaml` vs scene-router README prompts — single-source required.
- docker-compose: MinIO vs ClickHouse port 9000 clash; external `core_net` assumption.
- scene-gate frame-side notify (optional side-channel from camera-connector) still TBD; audio path is primary.
