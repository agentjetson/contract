package metadata

import (
	"context"
	"fmt"
	"time"

	"github.com/agentjetson/core/pkg/persistence"
)

// ClickHouseStore persists ObjectMeta via pkg/persistence (table object_meta).
// Schema is owned by seed/sql — this store never issues CREATE TABLE.
type ClickHouseStore struct {
	ch *persistence.Client
}

// NewClickHouseStore wraps an open persistence client.
// Caller owns the client lifetime (Close on shutdown).
func NewClickHouseStore(ch *persistence.Client) *ClickHouseStore {
	return &ClickHouseStore{ch: ch}
}

func (s *ClickHouseStore) Insert(ctx context.Context, r Record) error {
	labels := r.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	deleted := uint8(0)
	if r.Deleted {
		deleted = 1
	}
	return s.ch.InsertObjectMeta(ctx, persistence.ObjectMetaRow{
		ObjectID:       r.ObjectID,
		Kind:           r.Kind,
		Source:         r.Source,
		EventTS:        persistence.TruncateMS(r.EventTS),
		FrameID:        r.FrameID,
		TrackID:        r.TrackID,
		ContentType:    r.ContentType,
		SizeBytes:      r.SizeBytes,
		ChecksumSHA256: r.ChecksumSHA256,
		StorageKey:     r.StorageKey,
		Bucket:         r.Bucket,
		Labels:         labels,
		Deleted:        deleted,
	})
}

func (s *ClickHouseStore) Get(ctx context.Context, objectID string) (Record, error) {
	row := s.ch.Conn().QueryRow(ctx, `
		SELECT
			object_id, kind, source, event_ts, frame_id, track_id,
			content_type, size_bytes, checksum_sha256, storage_key, bucket,
			labels, created_at, deleted
		FROM object_meta
		WHERE object_id = ? AND deleted = 0
		ORDER BY created_at DESC
		LIMIT 1
	`, objectID)

	var rec Record
	var deleted uint8
	var labels map[string]string
	err := row.Scan(
		&rec.ObjectID, &rec.Kind, &rec.Source, &rec.EventTS, &rec.FrameID, &rec.TrackID,
		&rec.ContentType, &rec.SizeBytes, &rec.ChecksumSHA256, &rec.StorageKey, &rec.Bucket,
		&labels, &rec.CreatedAt, &deleted,
	)
	if err != nil {
		return Record{}, ErrNotFound
	}
	if labels == nil {
		labels = map[string]string{}
	}
	rec.Labels = labels
	rec.Deleted = deleted != 0
	if rec.Deleted {
		return Record{}, ErrNotFound
	}
	return rec, nil
}

func (s *ClickHouseStore) List(ctx context.Context, source, kind string, start, end time.Time, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// Dynamic filters; empty string means "any".
	query := `
		SELECT
			object_id, kind, source, event_ts, frame_id, track_id,
			content_type, size_bytes, checksum_sha256, storage_key, bucket,
			labels, created_at, deleted
		FROM object_meta
		WHERE deleted = 0
	`
	args := make([]any, 0, 6)
	if source != "" {
		query += ` AND source = ?`
		args = append(args, source)
	}
	if kind != "" && kind != "OBJECT_KIND_UNSPECIFIED" {
		query += ` AND kind = ?`
		args = append(args, kind)
	}
	if !start.IsZero() {
		query += ` AND event_ts >= ?`
		args = append(args, persistence.TruncateMS(start))
	}
	if !end.IsZero() {
		query += ` AND event_ts <= ?`
		args = append(args, persistence.TruncateMS(end))
	}
	query += ` ORDER BY event_ts DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.ch.Conn().Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("object_meta list: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var rec Record
		var deleted uint8
		var labels map[string]string
		if err := rows.Scan(
			&rec.ObjectID, &rec.Kind, &rec.Source, &rec.EventTS, &rec.FrameID, &rec.TrackID,
			&rec.ContentType, &rec.SizeBytes, &rec.ChecksumSHA256, &rec.StorageKey, &rec.Bucket,
			&labels, &rec.CreatedAt, &deleted,
		); err != nil {
			return nil, fmt.Errorf("object_meta scan: %w", err)
		}
		if labels == nil {
			labels = map[string]string{}
		}
		rec.Labels = labels
		rec.Deleted = deleted != 0
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// MarkDeleted sets deleted=1 via a lightweight mutation.
// Blob removal (hard delete) is handled by the storage backend, not here.
func (s *ClickHouseStore) MarkDeleted(ctx context.Context, objectID string) error {
	// Verify the row exists and is not already soft-deleted.
	if _, err := s.Get(ctx, objectID); err != nil {
		return err
	}
	err := s.ch.Conn().Exec(ctx, `
		ALTER TABLE object_meta
		UPDATE deleted = 1
		WHERE object_id = ? AND deleted = 0
	`, objectID)
	if err != nil {
		return fmt.Errorf("object_meta mark deleted: %w", err)
	}
	return nil
}
