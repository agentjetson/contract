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
camera / audio-client  ──transcript─► scene-gate
registered sources ──startup─────► EmitNow backend=profile
                                       │
                          EmitNow ─────┼──► IngestScene → cv.scene.*
                          ForwardVisual│    (edge scene-router unless SKIP_VISUAL)
                          Abstain ─────┘
```

---

## Config

| Path | Role |
|------|------|
| `domain/taxonomy.yaml` | Authoritative L1→L2 + specialists |
| `domain/camera_profiles.yaml` | Per-pattern profiles + audio_intents |
| `domain/camera_sources.yaml` | **Registered source ids** for profile emit |
| `TAXONOMY_PATH` / `CAMERA_PROFILES_PATH` / `CAMERA_SOURCES_PATH` | Env overrides |

### Fixed-role profile emit

List concrete units in `camera_sources.yaml`:

```yaml
sources:
  - id: front-cam-01
  - id: cabin-unit-01
```

On startup (when `EMIT_PROFILES=true`, default), for each id the gate runs **OnFrame**:

- Profile matches `skip_visual` + `l1_prior` → **EmitNow** (`backend=profile`, specialists from profile/taxonomy).
- Otherwise log ForwardVisual/Abstain and skip publish.

Optional refresh: `PROFILE_REFRESH_MIN=15` re-emits on that interval (0 = startup only).

Edge should still set `SKIP_VISUAL_SOURCES=front-*,cabin-*` so SigLIP does not run on those units.

### Source ID naming

| Deployment | Set `SOURCE` / source_id to | Profile match |
|------------|----------------------------|---------------|
| Officer body-cam | `bodycam-12` | `bodycam-*` (audio path) |
| Front plate cam | `front-cam-01` | `front-*` + list in camera_sources |
| Cabin / driver | `cabin-unit-01` | `cabin-*` + list in camera_sources |

Body-cam production: `SOURCE=bodycam-12` on audio-client (not `mic`).

---

## Behaviour

1. **Startup / refresh** — profile SceneResults for registered fixed-role sources.
2. **`audio.transcript`** — phrase → L2; high conf → EmitNow `backend=audio`.
3. **CLI** — `--source-id front-cam-01` or `--emit-profiles` for bring-up without NATS.

Dual scenes: if audio and visual both fire for the same source, prefer filtering by `backend` / confidence; fixed cams should not run visual at all.

---

## Run

```bash
# Live (NATS + ingest + profile emit + audio)
TAXONOMY_PATH=../domain/taxonomy.yaml \
CAMERA_PROFILES_PATH=../domain/camera_profiles.yaml \
CAMERA_SOURCES_PATH=../domain/camera_sources.yaml \
NATS_URL=nats://localhost:4222 \
INGEST_ADDR=localhost:50052 \
  go run ./cmd/scene-gate

# Bring-up single source
go run ./cmd/scene-gate --source-id front-cam-01
go run ./cmd/scene-gate --source-id bodycam-12 --simulate-audio "initiating traffic stop"

# Emit all registered profiles and exit (no NATS)
go run ./cmd/scene-gate --emit-profiles
```

| Variable | Default | Meaning |
|----------|---------|---------|
| `EMIT_PROFILES` | `true` | Startup profile emit from camera_sources |
| `PROFILE_REFRESH_MIN` | `0` | Minutes between re-emits; 0 = once |
| `CAMERA_SOURCES_PATH` | `/config/camera_sources.yaml` | Registered source ids |
| `NATS_URL` | `nats://localhost:4222` | |
| `INGEST_ADDR` | `localhost:50052` | |
| `CAMERA_PROFILES_PATH` | `/config/camera_profiles.yaml` | |
| `TAXONOMY_PATH` | `/config/taxonomy.yaml` | |

---

## Design principles

1. Same taxonomy as scene-router / temporal-classifier.
2. SceneResult remains the upstream situation contract.
3. Specialists stay pure consumers.
4. Fixed cameras: skip visual on edge + profile scene from gate.
5. Body-cam leans on speech when the officer announces the situation.
6. `Transcript.source` / camera `source_id` must match profile patterns.
