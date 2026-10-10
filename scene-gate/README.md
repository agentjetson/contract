# scene-gate (Go)

**Per-camera decision gate** for AgentJetson. Sits in core, upstream of edge scene-router, and decides whether to:

- **EmitNow** — publish a high-confidence `SceneResult` immediately (fixed-role camera profile or strong audio intent), or
- **ForwardVisual** — let the edge visual scene-router (+ optional temporal-classifier) handle the frame, or
- **Abstain** — drop / wait (e.g. audio_primary + no speech + skip_visual).

Same contracts as the rest of the stack: `domain/taxonomy.yaml`, specialists, `IngestScene` → `cv.scene.*`.

---

## Why Go in core

Scene-gate is policy/routing, not inference. No pixels, no ONNX. Taxonomy + profiles live next to ingest / aggregator; NATS + gRPC are already native here.

```
edge C++                          core (Go)
────────                          ─────────
scene-router / camera  ──cv.source.*──► scene-gate (dynamic register)
audio-client           ──transcript───► scene-gate
camera_sources.yaml    ──startup──────► EmitNow backend=profile (bootstrap)
                                       │
                          EmitNow ─────┼──► IngestScene → cv.scene.*
                          ForwardVisual│    (edge unless SKIP_VISUAL)
                          Abstain ─────┘
```

---

## Config

| Path | Role |
|------|------|
| `domain/taxonomy.yaml` | Authoritative L1→L2 + specialists |
| `domain/camera_profiles.yaml` | Per-pattern profiles + audio_intents |
| `domain/camera_sources.yaml` | Optional **bootstrap** source ids |
| `TAXONOMY_PATH` / `CAMERA_PROFILES_PATH` / `CAMERA_SOURCES_PATH` | Env overrides |

### Dynamic registration (`cv.source.*`)

Edge publishes JSON (or future protobuf `capture.v1.SourceEvent`):

```json
{"event":"up","source_id":"front-cam-01"}
{"event":"heartbeat","source_id":"front-cam-01"}
{"event":"down","source_id":"front-cam-01"}
```

Subjects (on stream `CV_EVENTS`):

| Subject | Meaning |
|---------|---------|
| `cv.source.up` | Source online — gate runs OnFrame; EmitNow if `skip_visual` profile |
| `cv.source.heartbeat` | Still alive — re-emit profile after `SOURCE_DEBOUNCE_SEC` |
| `cv.source.down` | Offline — drop from live set |

Gate best-effort updates the stream to include `cv.source.>` if missing.

`scene-router` with `SOURCE_ID` + `NATS_URL` publishes these (including skip-visual idle heartbeats).

### Static bootstrap

Still optional via `camera_sources.yaml` + `EMIT_PROFILES` for deploys before edge lifecycle exists.

### Source ID naming

| Deployment | Set `SOURCE` / source_id to | Profile match |
|------------|----------------------------|---------------|
| Officer body-cam | `bodycam-12` | `bodycam-*` (audio path) |
| Front plate cam | `front-cam-01` | `front-*` |
| Cabin / driver | `cabin-unit-01` | `cabin-*` |

Body-cam production: `SOURCE=bodycam-12` on audio-client (not `mic`).

---

## Behaviour

1. **Dynamic** — `cv.source.up` / heartbeat → profile SceneResult when profile says EmitNow.
2. **Startup bootstrap** — static `camera_sources.yaml` when `EMIT_PROFILES=true`.
3. **`audio.transcript`** — phrase → L2; high conf → EmitNow `backend=audio`.
4. **CLI** — `--source-id` / `--emit-profiles` without NATS.

---

## Run

```bash
TAXONOMY_PATH=../domain/taxonomy.yaml \
CAMERA_PROFILES_PATH=../domain/camera_profiles.yaml \
CAMERA_SOURCES_PATH=../domain/camera_sources.yaml \
NATS_URL=nats://localhost:4222 \
INGEST_ADDR=localhost:50052 \
DYNAMIC_SOURCES=true \
  go run ./cmd/scene-gate
```

| Variable | Default | Meaning |
|----------|---------|---------|
| `DYNAMIC_SOURCES` | `true` | Consume `cv.source.>` |
| `SOURCE_SUBJECT` | `cv.source.>` | JetStream filter |
| `SOURCE_DEBOUNCE_SEC` | `60` | Min seconds between profile emits per source |
| `EMIT_PROFILES` | `true` | Startup emit from camera_sources |
| `PROFILE_REFRESH_MIN` | `0` | Static list refresh; 0 = once |
| `STREAM_EVENTS` | `CV_EVENTS` | Must include `cv.source.>` |

---

## Design principles

1. Same taxonomy as scene-router / temporal-classifier.
2. SceneResult remains the upstream situation contract.
3. Specialists stay pure consumers.
4. Fixed cameras: skip visual on edge + profile scene from gate (static and/or dynamic).
5. Body-cam leans on speech when the officer announces the situation.
6. `Transcript.source` / camera `source_id` must match profile patterns.
