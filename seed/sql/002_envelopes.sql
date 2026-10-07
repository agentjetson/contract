-- AgentJetson Voice / core — ClickHouse schema notes
--
-- TWO layers exist today:
--
-- A) core clickhouse_consumer (src/clickhouse_consumer/main.cpp) writes TODAY:
--      cv_detections     ← NATS cv.alert
--      audio_transcripts ← NATS audio.transcript
--
-- B) query-service tools expect a richer set (ApplySchema in client_clickhouse.go):
--      cv_results        ← ALPR CapabilityResult (capability='alpr')  [NOT written by core yet]
--      cv_objects        ← ObjectEnvelope
--      cv_scenes         ← SceneResult
--      cv_detections     ← alerts (shape differs from core's table!)
--      audio_transcripts ← transcripts (shape differs from core's table!)
--
-- Until core is extended to write cv_results / cv_objects / cv_scenes and
-- the column shapes are unified, run query-service with DEMO_MODE=true or
-- adapt the Go queries to the core tables below.
--
-- ---------------------------------------------------------------------------
-- A) Tables created by core clickhouse_consumer (source of truth for live data)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS cv_detections
(
    frame_id         Int64,
    ts               DateTime64(9, 'UTC'),
    source           String,
    watchlist_hit    UInt8,
    matched_label    String,
    e2e_latency_ms   Float64,
    class_id         Int32,
    class_name       String,
    confidence       Float32,
    x1               Float32,
    y1               Float32,
    x2               Float32,
    y2               Float32,
    nats_seq         UInt64,
    ingested_at      DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree()
ORDER BY (source, ts, frame_id)
TTL toDateTime(ts) + INTERVAL 90 DAY;

CREATE TABLE IF NOT EXISTS audio_transcripts
(
    ts               DateTime64(9, 'UTC'),
    audio_start      DateTime64(9, 'UTC'),
    audio_end        DateTime64(9, 'UTC'),
    source           String,
    text             String,
    is_final         UInt8,
    confidence       Float32,
    language         String,
    speaker_id       String,
    e2e_latency_ms   Float64,
    nats_seq         UInt64,
    ingested_at      DateTime64(3, 'UTC') DEFAULT now64(3)
)
ENGINE = MergeTree()
ORDER BY (source, ts)
TTL toDateTime(ts) + INTERVAL 90 DAY;

-- ---------------------------------------------------------------------------
-- B) Target tables for query-service tools (aspirational / ApplySchema)
--    core must grow writers for cv_results (from cv.result.alpr),
--    cv_objects (from cv.object.*), cv_scenes (from cv.scene.*).
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS cv_results
(
    event_time     DateTime64(3),
    capability     LowCardinality(String),
    plate          String DEFAULT '',
    confidence     Float64 DEFAULT 0,
    track_id       String,
    camera_id      LowCardinality(String),
    vehicle_class  LowCardinality(String) DEFAULT '',
    make_model     String DEFAULT '',
    color          LowCardinality(String) DEFAULT '',
    scene_l1       LowCardinality(String) DEFAULT '',
    scene_l2       LowCardinality(String) DEFAULT '',
    raw_json       String DEFAULT ''
)
ENGINE = MergeTree()
ORDER BY (event_time, camera_id, track_id);

CREATE TABLE IF NOT EXISTS cv_objects
(
    event_time  DateTime64(3),
    class       LowCardinality(String),
    track_id    String,
    camera_id   LowCardinality(String),
    confidence  Float64,
    bbox        Array(Float32),
    scene_l1    LowCardinality(String),
    scene_l2    LowCardinality(String)
)
ENGINE = MergeTree()
ORDER BY (event_time, camera_id, track_id);

CREATE TABLE IF NOT EXISTS cv_scenes
(
    event_time   DateTime64(3),
    level1       LowCardinality(String),
    level2       LowCardinality(String),
    camera_id    LowCardinality(String),
    confidence   Float64,
    specialists  Array(String)
)
ENGINE = MergeTree()
ORDER BY (event_time, camera_id);
