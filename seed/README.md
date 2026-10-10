# ClickHouse seed

Canonical durable store for AgentJetson.

This folder is the **only** place tables are defined. Core's clickhouse_consumer
and voice-query-service's `ApplySchema` were competing implementations; they now
consume this schema.

## Layout

| File                    | Role                                      |
| ----------------------- | ----------------------------------------- |
| `sql/001_database.sql`  | Database notes                            |
| `sql/002_tables.sql`    | Canonical MergeTree tables (proto-aligned)|
| `sql/003_views.sql`     | `query_*` views for the Go voice-query-service  |
| `sql/004_seed.sql`      | Demo rows (`source LIKE 'seed.%'`)        |
| `WRITE_SPEC.md`         | How the consumer maps NATS → rows         |

## Bring-up (ClickHouse only)

From the core repo root:

```bash
make up          # docker compose up clickhouse
make schema      # idempotent DDL (also applied on first boot)
make seed        # demo plates / objects / scenes / transcripts
```

Native: `localhost:9000`. HTTP: `localhost:8123`.
Auth: `default` / `pass`. Database: `default` (same env as core).

## Domain models persisted

| Producer            | Proto                 | Table             |
| ------------------- | --------------------- | ----------------- |
| object-classifier   | ObjectEnvelope        | `cv_objects`      |
| alpr-consumer (+…)  | CapabilityResult      | `cv_results`      |
| scene-router        | SceneResult           | `cv_scenes`       |
| temporal-classifier | SceneResult (refined) | `cv_scenes`       |
| aggregator          | Alert                 | `cv_detections`   |
| audio-client        | Transcript            | `audio_transcripts` |

Classifiers and transcribers do not own tables. They publish contracts; the
ClickHouse adapter writes rows.

## Re-seed

`make seed` deletes `source LIKE 'seed.%'` then inserts the Mercedes / plate
fixture used by voice-query-service DEMO_MODE.
