package persistence

import (
	"context"
	"fmt"
)

// InsertObjects batch-writes cv_objects rows.
func (c *Client) InsertObjects(ctx context.Context, rows []ObjectRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO cv_objects (
			frame_id, ts, source, class_name, class_id, confidence,
			x1, y1, x2, y2, track_id, frame_width, frame_height,
			capture_latency_ms, scene_l1, scene_l2, nats_seq, labels
		)`)
	if err != nil {
		return fmt.Errorf("prepare cv_objects: %w", err)
	}
	for _, r := range rows {
		labels := r.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		if err := batch.Append(
			r.FrameID, r.TS, r.Source, r.ClassName, r.ClassID, r.Confidence,
			r.X1, r.Y1, r.X2, r.Y2, r.TrackID, r.FrameWidth, r.FrameHeight,
			r.CaptureLatencyMs, r.SceneL1, r.SceneL2, r.NatsSeq, labels,
		); err != nil {
			return fmt.Errorf("append cv_objects: %w", err)
		}
	}
	return batch.Send()
}

// InsertResults batch-writes cv_results rows.
func (c *Client) InsertResults(ctx context.Context, rows []ResultRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO cv_results (
			frame_id, ts, source, capability, track_id, class_name, confidence,
			x1, y1, x2, y2, ocr_text, ocr_confidence,
			plate_x1, plate_y1, plate_x2, plate_y2, processing_ms,
			vehicle_class, make_model, color, scene_l1, scene_l2,
			nats_seq, attributes, labels
		)`)
	if err != nil {
		return fmt.Errorf("prepare cv_results: %w", err)
	}
	for _, r := range rows {
		attrs := r.Attributes
		if attrs == nil {
			attrs = map[string]string{}
		}
		labels := r.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		if err := batch.Append(
			r.FrameID, r.TS, r.Source, r.Capability, r.TrackID, r.ClassName, r.Confidence,
			r.X1, r.Y1, r.X2, r.Y2, r.OCRText, r.OCRConfidence,
			r.PlateX1, r.PlateY1, r.PlateX2, r.PlateY2, r.ProcessingMs,
			r.VehicleClass, r.MakeModel, r.Color, r.SceneL1, r.SceneL2,
			r.NatsSeq, attrs, labels,
		); err != nil {
			return fmt.Errorf("append cv_results: %w", err)
		}
	}
	return batch.Send()
}

// InsertScenes batch-writes cv_scenes rows.
func (c *Client) InsertScenes(ctx context.Context, rows []SceneRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO cv_scenes (
			frame_id, ts, source, level1, level2,
			level1_confidence, level2_confidence, specialists,
			backend, temporal_requested, notes, nats_seq
		)`)
	if err != nil {
		return fmt.Errorf("prepare cv_scenes: %w", err)
	}
	for _, r := range rows {
		specs := r.Specialists
		if specs == nil {
			specs = []string{}
		}
		if err := batch.Append(
			r.FrameID, r.TS, r.Source, r.Level1, r.Level2,
			r.Level1Confidence, r.Level2Confidence, specs,
			r.Backend, r.TemporalRequested, r.Notes, r.NatsSeq,
		); err != nil {
			return fmt.Errorf("append cv_scenes: %w", err)
		}
	}
	return batch.Send()
}

// InsertDetections batch-writes cv_detections rows.
func (c *Client) InsertDetections(ctx context.Context, rows []DetectionRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO cv_detections (
			frame_id, ts, source, watchlist_hit, matched_label, e2e_latency_ms,
			class_id, class_name, confidence, x1, y1, x2, y2, track_id, nats_seq, labels
		)`)
	if err != nil {
		return fmt.Errorf("prepare cv_detections: %w", err)
	}
	for _, r := range rows {
		labels := r.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		if err := batch.Append(
			r.FrameID, r.TS, r.Source, r.WatchlistHit, r.MatchedLabel, r.E2ELatencyMs,
			r.ClassID, r.ClassName, r.Confidence, r.X1, r.Y1, r.X2, r.Y2, r.TrackID, r.NatsSeq, labels,
		); err != nil {
			return fmt.Errorf("append cv_detections: %w", err)
		}
	}
	return batch.Send()
}

// InsertTranscripts batch-writes audio_transcripts rows.
func (c *Client) InsertTranscripts(ctx context.Context, rows []TranscriptRow) error {
	if len(rows) == 0 {
		return nil
	}
	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO audio_transcripts (
			ts, audio_start, audio_end, source, text, is_final, confidence,
			language, speaker_id, e2e_latency_ms, nats_seq, labels
		)`)
	if err != nil {
		return fmt.Errorf("prepare audio_transcripts: %w", err)
	}
	for _, r := range rows {
		labels := r.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		if err := batch.Append(
			r.TS, r.AudioStart, r.AudioEnd, r.Source, r.Text, r.IsFinal, r.Confidence,
			r.Language, r.SpeakerID, r.E2ELatencyMs, r.NatsSeq, labels,
		); err != nil {
			return fmt.Errorf("append audio_transcripts: %w", err)
		}
	}
	return batch.Send()
}

// InsertObjectMeta writes a single object_meta row (object-storage path).
func (c *Client) InsertObjectMeta(ctx context.Context, r ObjectMetaRow) error {
	batch, err := c.conn.PrepareBatch(ctx, `
		INSERT INTO object_meta (
			object_id, kind, source, event_ts, frame_id, track_id,
			content_type, size_bytes, checksum_sha256, storage_key, bucket,
			labels, deleted
		)`)
	if err != nil {
		return fmt.Errorf("prepare object_meta: %w", err)
	}
	labels := r.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	if err := batch.Append(
		r.ObjectID, r.Kind, r.Source, r.EventTS, r.FrameID, r.TrackID,
		r.ContentType, r.SizeBytes, r.ChecksumSHA256, r.StorageKey, r.Bucket,
		labels, r.Deleted,
	); err != nil {
		return fmt.Errorf("append object_meta: %w", err)
	}
	return batch.Send()
}
