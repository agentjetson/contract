-- AgentJetson ClickHouse — voice-query-service compatibility views
--
-- Canonical tables use proto field names (ts, source, class_name, ocr_text).
-- voice-query-service historically queried a competing shape
-- (event_time, camera_id, plate, class, ...). These views are the adapter:
-- the Go service should SELECT from query_* instead of CREATE TABLE of its own.
--
-- ApplySchema in voice-query-service/internal/clickhouse MUST go away.

CREATE OR REPLACE VIEW query_cv_results AS
SELECT
    ts                  AS event_time,
    capability,
    ocr_text            AS plate,
    ocr_confidence      AS confidence,
    toString(track_id)  AS track_id,
    source              AS camera_id,
    if(vehicle_class = '', class_name, vehicle_class) AS vehicle_class,
    make_model,
    color,
    scene_l1,
    scene_l2,
    frame_id,
    class_name
FROM cv_results;

CREATE OR REPLACE VIEW query_cv_objects AS
SELECT
    ts                  AS event_time,
    class_name          AS class,
    toString(track_id)  AS track_id,
    source              AS camera_id,
    confidence,
    [x1, y1, x2, y2]    AS bbox,
    scene_l1,
    scene_l2,
    frame_id,
    class_id
FROM cv_objects;

CREATE OR REPLACE VIEW query_cv_scenes AS
SELECT
    ts                  AS event_time,
    level1,
    level2,
    source              AS camera_id,
    level2_confidence   AS confidence,
    specialists,
    frame_id,
    backend,
    temporal_requested
FROM cv_scenes;

CREATE OR REPLACE VIEW query_cv_detections AS
SELECT
    concat(source, ':', toString(frame_id), ':', toString(track_id)) AS id,
    class_name          AS class,
    source              AS camera_id,
    toString(track_id)  AS track_id,
    confidence,
    ts                  AS event_time,
    matched_label       AS metadata,
    watchlist_hit,
    frame_id
FROM cv_detections;

CREATE OR REPLACE VIEW query_audio_transcripts AS
SELECT
    ts                  AS event_time,
    text,
    speaker_id          AS speaker,
    source              AS device_id,
    confidence,
    is_final,
    language
FROM audio_transcripts;
