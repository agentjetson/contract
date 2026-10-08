-- AgentJetson ClickHouse — demo rows
-- Safe to re-run: deletes previous seed.* sources first.
-- Matches the voice-query-service DEMO_MODE fixture (plate 7ABC123, Mercedes, track 77).

ALTER TABLE cv_objects          DELETE WHERE source LIKE 'seed.%';
ALTER TABLE cv_results          DELETE WHERE source LIKE 'seed.%';
ALTER TABLE cv_scenes           DELETE WHERE source LIKE 'seed.%';
ALTER TABLE cv_detections       DELETE WHERE source LIKE 'seed.%';
ALTER TABLE audio_transcripts   DELETE WHERE source LIKE 'seed.%';

INSERT INTO cv_scenes (
    frame_id, ts, source, level1, level2,
    level1_confidence, level2_confidence, specialists, backend, temporal_requested, notes
) VALUES
    (1001, now64(3) - INTERVAL 5 SECOND,  'seed.cam-01', 'roadway',  'driving', 0.97, 0.93, ['alpr','speed'], 'siglip2', 0, ''),
    (2001, now64(3) - INTERVAL 12 SECOND, 'seed.cam-02', 'sidewalk', 'walking', 0.91, 0.84, ['person_attr'],   'siglip2', 0, '');

INSERT INTO cv_objects (
    frame_id, ts, source, class_name, class_id, confidence,
    x1, y1, x2, y2, track_id, frame_width, frame_height, scene_l1, scene_l2
) VALUES
    (1001, now64(3) - INTERVAL 8 SECOND,  'seed.cam-01', 'car',    2, 0.96, 120, 200, 480, 410, 77, 1920, 1080, 'roadway',  'driving'),
    (1008, now64(3) - INTERVAL 22 SECOND, 'seed.cam-01', 'truck',  7, 0.93,  40, 180, 620, 500, 81, 1920, 1080, 'roadway',  'driving'),
    (2004, now64(3) - INTERVAL 15 SECOND, 'seed.cam-02', 'person', 0, 0.89, 800, 240, 920, 720, 12, 1920, 1080, 'sidewalk', 'walking');

INSERT INTO cv_results (
    frame_id, ts, source, capability, track_id, class_name, confidence,
    x1, y1, x2, y2, ocr_text, ocr_confidence,
    vehicle_class, make_model, color, scene_l1, scene_l2
) VALUES
    (1001, now64(3) - INTERVAL 8 SECOND,  'seed.cam-01', 'alpr', 77, 'car',   0.96, 120, 200, 480, 410, '7ABC123', 0.94, 'car',   'Mercedes C-Class', 'silver', 'roadway', 'driving'),
    (1008, now64(3) - INTERVAL 22 SECOND, 'seed.cam-01', 'alpr', 81, 'truck', 0.93,  40, 180, 620, 500, '9XYZ456', 0.88, 'truck', 'Ford F-150',       'blue',   'roadway', 'driving'),
    (2010, now64(3) - INTERVAL 45 SECOND, 'seed.cam-02', 'alpr', 65, 'car',   0.91, 300, 220, 700, 480, '4DEF789', 0.91, 'car',   'Toyota Camry',     'white',  'roadway', 'stopped');

INSERT INTO cv_detections (
    frame_id, ts, source, watchlist_hit, matched_label, e2e_latency_ms,
    class_id, class_name, confidence, x1, y1, x2, y2, track_id
) VALUES
    (1001, now64(3) - INTERVAL 8 SECOND,  'seed.cam-01', 1, 'watchlist', 42.0, 2, 'car',    0.96, 120, 200, 480, 410, 77),
    (1008, now64(3) - INTERVAL 22 SECOND, 'seed.cam-01', 0, '',          38.0, 7, 'truck',  0.93,  40, 180, 620, 500, 81),
    (2004, now64(3) - INTERVAL 15 SECOND, 'seed.cam-02', 0, '',          29.0, 0, 'person', 0.89, 800, 240, 920, 720, 12);

INSERT INTO audio_transcripts (
    ts, audio_start, audio_end, source, text, is_final, confidence, language, speaker_id, e2e_latency_ms
) VALUES
    (now64(3) - INTERVAL 6 SECOND,  now64(3) - INTERVAL 8 SECOND,  now64(3) - INTERVAL 6 SECOND,  'seed.mic-01', 'check the license plate of the Mercedes that just passed', 1, 0.92, 'en', 'officer-1', 180),
    (now64(3) - INTERVAL 20 SECOND, now64(3) - INTERVAL 22 SECOND, now64(3) - INTERVAL 20 SECOND, 'seed.mic-01', 'any trucks on cam one in the last minute', 1, 0.88, 'en', 'officer-1', 160);
