# ClickHouse writer spec

One durable store. One writer. Schema is applied by `make schema` (this repo),
not by `core/src/clickhouse_consumer` and not by `voice-query-service` `ApplySchema`.

## Subscribe

| JetStream subject     | Proto                         | Table               | Rows                          |
| --------------------- | ----------------------------- | ------------------- | ----------------------------- |
| `cv.object.>`         | `detection.v1.ObjectEnvelope` | `cv_objects`        | 1 per envelope                |
| `cv.result.>`         | `detection.v1.CapabilityResult` | `cv_results`      | 1 per result                  |
| `cv.scene.>`          | `scene.v1.SceneResult`        | `cv_scenes`         | 1 per result                  |
| `cv.alert`            | `detection.v1.Alert`          | `cv_detections`     | 1 per `Detection` (denorm)    |
| `audio.transcript`    | `audio.v1.Transcript`         | `audio_transcripts` | 1 per transcript              |

Do **not** persist `crop_jpeg`, `PreparedCrop.jpeg`, or `FrameEnvelope.payload`.

## Column mapping (canonical)

Time → `ts DateTime64(3, 'UTC')` (truncate proto nanos).
Identity → proto `source` (camera / mic / file). Never rename to `camera_id` in the table.
Track → proto `track_id` as `Int32`.
Box → `x1, y1, x2, y2` (not `bbox` array, not `bbox_w/h`).
ALPR plate → `ocr_text` / `ocr_confidence`.
Capability attributes `make`, `model`, `color`, `vehicle_class` → denormalized columns on `cv_results`.
Scene labels on objects/results (`scene_l1`, `scene_l2`) are filled by the writer when the envelope labels carry them; otherwise left empty (aggregator may backfill later).

`nats_seq` from JetStream metadata. `ingested_at` is `now64(3)` (table default).

## Who must stop creating tables

- `core/src/clickhouse_consumer/main.cpp` — drop the inline `CREATE TABLE` strings; assume this schema exists. Add writers for objects / results / scenes.
- `voice/voice-query-service/internal/clickhouse/client_clickhouse.go` `ApplySchema` — delete it. Query the `query_*` views (see `003_views.sql`).
- Core `docker-compose.yml` — remove the `clickhouse` service; point `CLICKHOUSE_HOST` at this compose.

## video_server

Unchanged. It already selects physical columns from `cv_detections`:

```
SELECT frame_id, class_id, class_name, confidence, x1, y1, x2, y2
FROM cv_detections
WHERE source = ... AND frame_id >= ...
```

## voice-query-service SQL swap

| Old table            | New view                 |
| -------------------- | ------------------------ |
| `cv_results`         | `query_cv_results`       |
| `cv_objects`         | `query_cv_objects`       |
| `cv_scenes`          | `query_cv_scenes`        |
| `cv_detections`      | `query_cv_detections`    |
| `audio_transcripts`  | `query_audio_transcripts`|

View column names match the current Go `Scan` targets (`event_time`, `camera_id`, `plate`, ...).
