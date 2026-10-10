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
camera / audio-client  ──frames──► (optional light notify — TBD)
                     ──transcript─► scene-gate
                                       │
                          EmitNow ─────┼──► IngestScene → cv.scene.*
                          ForwardVisual│    (edge scene-router still runs unless skip_visual)
                          Abstain ─────┘
```

---

## Config

| Path | Role |
|------|------|
| `domain/taxonomy.yaml` | Authoritative L1→L2 + specialists |
| `domain/camera_profiles.yaml` | Per-`source_id` profiles + audio_intents |
| `TAXONOMY_PATH` / `CAMERA_PROFILES_PATH` | Env overrides |

### Profile fields

```yaml
profiles:
  - match: "front-*"
    l1_prior: roadway
    specialists_always: [alpr, speed, vehicle_attr]
    specialists_allowed: [alpr, speed, vehicle_attr]
    skip_visual: true
    audio_primary: false
    visual_abstain_threshold: 0.60
    audio_confidence_threshold: 0.70
```

`audio_intents` map phrases → L2 (case-insensitive, first hit wins).

### Source ID naming (critical)

Profiles match on **`Transcript.source`** / camera `source_id` using exact or trailing-`*` patterns.

| Deployment | Set `SOURCE` / source_id to | Profile match |
|------------|----------------------------|---------------|
| Officer body-cam | `bodycam-12`, `bodycam-unit-7` | `bodycam-*` |
| Front plate cam | `front-cam-01` | `front-*` |
| Cabin / driver | `cabin-cruiser-3` | `cabin-*` |
| Dashcam outward | `dashcam-car-9` | `dashcam-*` |

If audio-client uses the default `SOURCE=mic`, **no profile matches** and audio intent never short-circuits. Body-cam deployments must set:

```bash
SOURCE=bodycam-12   # or bodycam-<unit-id>
```

Same id should be used by camera-connector for that unit so visual and audio correlate.

---

## Behaviour

### Production path (live service)

The long-running service **only** consumes `audio.transcript` (JetStream `AUDIO_EVENTS`).

1. Resolve profile for `Transcript.source`.
2. Match phrase → L2; if confidence ≥ profile threshold → **EmitNow** (`backend="audio"`).
3. Else **ForwardVisual** (or **Abstain** if `audio_primary && skip_visual`).

### Bring-up / CLI only (`--source-id`)

```bash
go run ./cmd/scene-gate --source-id front-cam-01
go run ./cmd/scene-gate --source-id bodycam-12 --simulate-audio "initiating traffic stop"
```

`--source-id` without `--simulate-audio` runs **OnFrame** (profile prior + `skip_visual`). That path is **bring-up / test only**. There is no production frame side-channel yet, so fixed-role cameras (`front-*`, `cabin-*`) do **not** auto-emit profile SceneResults in the live loop.

Until a camera registration / heartbeat notify exists:

- Rely on **edge scene-router** honouring `SKIP_VISUAL_SOURCES` / profiles for fixed cams (see scene-router), **or**
- Manually emit once at deploy with the CLI and treat it as a config smoke test.

### Dual SceneResult

If the gate EmitNows from audio **and** scene-router still runs vision on the same source, downstream may see two scenes (`backend=audio` vs `backend=siglip2`). Prefer edge skip for `skip_visual` sources; filter by backend/confidence if both appear.

---

## Run

```bash
# With core docker-compose (NATS + ingest up)
TAXONOMY_PATH=../domain/taxonomy.yaml \
CAMERA_PROFILES_PATH=../domain/camera_profiles.yaml \
NATS_URL=nats://localhost:4222 \
INGEST_ADDR=localhost:50052 \
  go run ./cmd/scene-gate

# Bring-up without live audio
go run ./cmd/scene-gate --source-id front-cam-01
go run ./cmd/scene-gate --source-id bodycam-12 --simulate-audio "initiating traffic stop"
```

Env:

| Variable | Default | Meaning |
|----------|---------|---------|
| `NATS_URL` | `nats://localhost:4222` | JetStream |
| `INGEST_ADDR` | `localhost:50052` | gRPC ingest for IngestScene |
| `TAXONOMY_PATH` | `/config/taxonomy.yaml` | |
| `CAMERA_PROFILES_PATH` | `/config/camera_profiles.yaml` | |
| `AUDIO_SUBJECT` | `audio.transcript` | |
| `STREAM_AUDIO` | `AUDIO_EVENTS` | |

---

## Design principles

1. Same taxonomy as scene-router / temporal-classifier — no divergent tables.
2. SceneResult remains the upstream situation contract.
3. Specialists stay pure consumers.
4. Fixed cameras stay cheap (skip visual on edge + profile prior).
5. Body-cam leans on speech when the officer already announces the situation.
6. `Transcript.source` / camera `source_id` must match profile patterns.
