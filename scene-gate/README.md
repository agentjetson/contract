# scene-gate (Go)

**Per-camera decision gate** for AgentJetson. Sits in core, upstream of edge scene-router, and decides whether to:

- **EmitNow** — publish a high-confidence `SceneResult` immediately (fixed-role camera profile or strong audio intent), or
- **ForwardVisual** — let the edge visual scene-router (+ optional temporal-classifier) handle the frame, or
- **Abstain** — drop / wait (e.g. audio_primary + no speech + skip_visual).

Same contracts as the rest of the stack: `domain/taxonomy.yaml`, specialists, `IngestScene` → `cv.scene.*`.

---

## Why Go in core

Scene-gate is policy/routing, not inference. No pixels, no ONNX. Taxonomy + profiles live next to ingest / aggregator; NATS + gRPC are already native here. The C++ repo (`agentjetson/scene-gate`) is a disposable prototype.

```
edge C++                          core (Go)
────────                          ─────────
camera / audio-client  ──frames──► (optional light notify)
                     ──transcript─► scene-gate
                                       │
                          EmitNow ─────┼──► IngestScene → cv.scene.*
                          ForwardVisual│    (edge scene-router still runs)
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

---

## Behaviour

1. **On `audio.transcript`** (NATS):
   - Resolve profile for `source`.
   - Match phrase → L2; if confidence ≥ profile threshold → **EmitNow** with specialists from taxonomy/profile, `backend="audio"`.
   - Else fall through (or Abstain if `audio_primary && skip_visual`).

2. **On frame / source notification** (optional side-channel or CLI):
   - If `skip_visual: true` + `l1_prior` → **EmitNow** with neutral L2 under that prior, `backend="profile"`.
   - Else → **ForwardVisual** (edge scene-router owns the work).

3. SceneResult is published via gRPC to ingest (`IngestScene`) or directly to NATS when configured.

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
4. Fixed cameras stay cheap (skip visual).
5. Body-cam leans on speech when the officer already announces the situation.
