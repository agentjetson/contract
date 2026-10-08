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

subgraph Core Services
ING[ingest_server<br/>gRPC → NATS]
NP[nats_publisher<br/>JetStream]
AGG[aggregator<br/>correlate + watchlist]
CONS[consumer / clickhouse_consumer]
CH[(ClickHouse)]
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

subgraph Contract["agentjetson/contract (this repo)"]
VQS[query-service<br/>Go · HTTP :8080 · MCP]
OBS[object-storage<br/>Go · HTTP :8081 · gRPC :50055]
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
NP -->|cv.object.*| ALPR
NP -->|cv.object.* + cv.result.* + cv.scene.*| AGG
ALPR -->|CapabilityResult<br/>cv.result.alpr| NP
AGG -->|Alert<br/>cv.alert| CONS
CONS --> CH

RFD -.->|primary model| OC
RFD -.->|plate model| ALPR

AC -->|IngestTranscript / audio.transcript| ING
AC -.->|optional shared STT| VA
AC -.->|optional PutObject| OBS
VA -->|transcript| LLM
VA -->|POST /v1/query/* libcurl| VQS
LLM -.->|future tool-calls| VQS
VQS -->|read-only queries| CH
VQS -.->|optional live peek| NP
OBS -->|object_meta| CH
OBS -->|blobs| MinIO[(MinIO / S3)]

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
```

## Subject hierarchy (JetStream stream `CV_EVENTS`)

| Subject pattern       | Producer          | Consumer(s)              | Payload              |
|-----------------------|-------------------|--------------------------|----------------------|
| `cv.object.<class>`   | recorder (object-classifier) | alpr_consumer, aggregator | `ObjectEnvelope`     |
| `cv.result.<capability>` | specialists (alpr, …) | aggregator            | `CapabilityResult`   |
| `cv.alert`            | aggregator        | consumer, clickhouse, …  | `Alert`              |

Legacy stream `CV_ALERTS` still captures `cv.alert` for existing consumers.

## Flow

```
Cameras → recorder (slim Pipeline)
            │ detect + track + crop JPEG
            ▼
        IngestObject (gRPC)
            ▼
        NATS  cv.object.car / cv.object.person / …
            │
            ├──────────────────► alpr_consumer
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
                                   ├─► consumer (demo)
                                   └─► clickhouse_consumer
```

## Components

### Recorder (camera-connector + object-classifier (ConsumerPipeline))
- Capture lives in `camera-connector`; detect+track lives in `object-classifier`.
- Capture, primary detection, tracking only.
- Emits one `ObjectEnvelope` per tracked object of interest (with optional JPEG crop).
- **No** plate OCR, **no** watchlist.

### ALPR consumer ([alpr-consumer](https://github.com/agentjetson/alpr-consumer)) — specialist template
- Pulls `cv.object.*`, filters to vehicle classes.
- Runs plate ROI + OCR (mock today; drop in real LPRNet later).
- Publishes `CapabilityResult` on `cv.result.alpr`.

### Aggregator (`src/aggregator`)
- Correlates objects + capability results in a short time window.
- Applies watchlist (class + plate text).
- Emits final `Alert` on `cv.alert`.

## Adding a new specialist (e.g. vehicle colour)

1. Copy `src/alpr_consumer` → `src/vehicle_attr_consumer`.
2. Change filter / capability string / inference.
3. Publish to `cv.result.vehicle_attr`.
4. Extend aggregator to merge the new attributes into the Alert (or keep them as side-channel results).

## Incremental path status

1. ✅ Extract plate logic into dedicated ALPR consumer
2. ✅ Recorder publishes object envelopes (class + bbox + crop)
3. ✅ Subject hierarchy `cv.object.>`, `cv.result.>`, `cv.alert`
5. ✅ Watchlist / alert assembly moved to aggregator
