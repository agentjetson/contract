-- AgentJetson ClickHouse — canonical tables
-- Single source of truth. Core clickhouse_consumer and voice-query-service MUST NOT
-- CREATE TABLE on their own; they consume this schema.
--
-- Mapping: proto → NATS subject → table  (see seed/WRITE_SPEC.md)
--
--   detection.v1.ObjectEnvelope     cv.object.>     cv_objects
--   detection.v1.CapabilityResult   cv.result.>     cv_results
--   scene.v1.SceneResult            cv.scene.>      cv_scenes
--   detection.v1.Alert              cv.alert        cv_detections
--   audio.v1.Transcript             audio.transcript audio_transcripts
--
-- JPEG / crop bytes are transport-only and are never stored.
-- voice-query-service reads the query_* VIEWS in 003_views.sql (column aliases).
-- video_server reads cv_detections physical columns (frame_id, class_id, ...).

-- ---------------------------------------------------------------------------
-- cv_objects ← ObjectEnvelope (classifier / crop-preparator)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cv_objects
(
    frame_id            Int64,
    ts                  DateTime64(3, 'UTC'),
    source              LowCardinality(String),
    class_name          LowCardinality(String),
    class_id            Int32,
    confidence          Float32,
    x1                  Float32,
    y1                  Float32,
    x2                  Float32,
    y2                  Float32,
    track_id            Int32,
    frame_width         Int32 DEFAULT 0,
    frame_height        Int32 DEFAULT 0,
    capture_latency_ms  Float64 DEFAULT 0,
    scene_l1            LowCardinality(String) DEFAULT '',
    scene_l2            LowCardinality(String) DEFAULT '',
    nats_seq            UInt64 DEFAULT 0,
    ingested_at         DateTime64(3, 'UTC') DEFAULT now64(3),
    labels              Map(LowCardinality(String), String) DEFAULT map()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (source, class_name, ts, track_id)
TTL toDateTime(ts) + INTERVAL 90 DAY;

-- ---------------------------------------------------------------------------
-- cv_results ← CapabilityResult (alpr-consumer and future specialists)
-- ocr_text is the plate for capability='alpr'.
-- make_model / color / vehicle_class are denormalized from attributes{}
-- so plate queries do not parse maps.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cv_results
(
    frame_id            Int64,
    ts                  DateTime64(3, 'UTC'),
    source              LowCardinality(String),
    capability          LowCardinality(String),
    track_id            Int32,
    class_name          LowCardinality(String),
    confidence          Float32 DEFAULT 0,
    x1                  Float32 DEFAULT 0,
    y1                  Float32 DEFAULT 0,
    x2                  Float32 DEFAULT 0,
    y2                  Float32 DEFAULT 0,
    ocr_text            String DEFAULT '',
    ocr_confidence      Float32 DEFAULT 0,
    plate_x1            Float32 DEFAULT 0,
    plate_y1            Float32 DEFAULT 0,
    plate_x2            Float32 DEFAULT 0,
    plate_y2            Float32 DEFAULT 0,
    processing_ms       Float64 DEFAULT 0,
    vehicle_class       LowCardinality(String) DEFAULT '',
    make_model          String DEFAULT '',
    color               LowCardinality(String) DEFAULT '',
    scene_l1            LowCardinality(String) DEFAULT '',
    scene_l2            LowCardinality(String) DEFAULT '',
    nats_seq            UInt64 DEFAULT 0,
    ingested_at         DateTime64(3, 'UTC') DEFAULT now64(3),
    attributes          Map(LowCardinality(String), String) DEFAULT map(),
    labels              Map(LowCardinality(String), String) DEFAULT map()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (capability, source, ts, track_id)
TTL toDateTime(ts) + INTERVAL 90 DAY;

-- ---------------------------------------------------------------------------
-- cv_scenes ← SceneResult (scene-router / temporal-classifier)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cv_scenes
(
    frame_id            Int64,
    ts                  DateTime64(3, 'UTC'),
    source              LowCardinality(String),
    level1              LowCardinality(String),
    level2              LowCardinality(String) DEFAULT '',
    level1_confidence   Float32 DEFAULT 0,
    level2_confidence   Float32 DEFAULT 0,
    specialists         Array(LowCardinality(String)) DEFAULT [],
    backend             LowCardinality(String) DEFAULT '',
    temporal_requested  UInt8 DEFAULT 0,
    notes               String DEFAULT '',
    nats_seq            UInt64 DEFAULT 0,
    ingested_at         DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (source, level1, ts)
TTL toDateTime(ts) + INTERVAL 90 DAY;

-- ---------------------------------------------------------------------------
-- cv_detections ← Alert (aggregator), one row per Detection
-- Column names on the box fields stay stable for video_server:
--   SELECT frame_id, class_id, class_name, confidence, x1, y1, x2, y2
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS cv_detections
(
    frame_id            Int64,
    ts                  DateTime64(3, 'UTC'),
    source              LowCardinality(String),
    watchlist_hit       UInt8 DEFAULT 0,
    matched_label       String DEFAULT '',
    e2e_latency_ms      Float64 DEFAULT 0,
    class_id            Int32 DEFAULT -1,
    class_name          LowCardinality(String) DEFAULT '',
    confidence          Float32 DEFAULT 0,
    x1                  Float32 DEFAULT 0,
    y1                  Float32 DEFAULT 0,
    x2                  Float32 DEFAULT 0,
    y2                  Float32 DEFAULT 0,
    track_id            Int32 DEFAULT 0,
    nats_seq            UInt64 DEFAULT 0,
    ingested_at         DateTime64(3, 'UTC') DEFAULT now64(3),
    labels              Map(LowCardinality(String), String) DEFAULT map()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (source, ts, frame_id)
TTL toDateTime(ts) + INTERVAL 90 DAY;

-- ---------------------------------------------------------------------------
-- audio_transcripts ← Transcript (audio-client)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audio_transcripts
(
    ts                  DateTime64(3, 'UTC'),
    audio_start         DateTime64(3, 'UTC'),
    audio_end           DateTime64(3, 'UTC'),
    source              LowCardinality(String),
    text                String,
    is_final            UInt8 DEFAULT 1,
    confidence          Float32 DEFAULT 0,
    language            LowCardinality(String) DEFAULT 'en',
    speaker_id          String DEFAULT '',
    e2e_latency_ms      Float64 DEFAULT 0,
    nats_seq            UInt64 DEFAULT 0,
    ingested_at         DateTime64(3, 'UTC') DEFAULT now64(3),
    labels              Map(LowCardinality(String), String) DEFAULT map()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(ts)
ORDER BY (source, ts)
TTL toDateTime(ts) + INTERVAL 90 DAY;

-- Object-storage metadata table.
-- Written by object-storage service after every successful Put*.
-- Queried by voice-query-service (and future correlation jobs) to join
-- blobs back to cv.object.*, cv.scene.*, audio.transcript, cv.alert.

CREATE TABLE IF NOT EXISTS object_meta
(
    object_id       String,
    kind            LowCardinality(String),   -- audio_transcript | camera_recording | …
    source          String,
    event_ts        DateTime64(3, 'UTC'),
    frame_id        Int64 DEFAULT 0,
    track_id        Int32 DEFAULT 0,
    content_type    LowCardinality(String),
    size_bytes      Int64,
    checksum_sha256 String,
    storage_key     String,
    bucket          String,
    labels          Map(String, String),
    created_at      DateTime64(3, 'UTC') DEFAULT now64(3),
    deleted         UInt8 DEFAULT 0
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(event_ts)
ORDER BY (source, kind, event_ts, object_id)
TTL toDateTime(event_ts) + INTERVAL 365 DAY DELETE
SETTINGS index_granularity = 8192;
-- Secondary index for frame_id correlation (common join path).
ALTER TABLE object_meta
    ADD INDEX IF NOT EXISTS idx_frame_id frame_id TYPE minmax GRANULARITY 4;
