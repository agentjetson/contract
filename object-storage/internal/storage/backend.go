package storage

import (
	"context"
	"io"
	"time"
)

// ObjectInfo is the backend-agnostic description of a stored blob.
type ObjectInfo struct {
	Key          string
	Bucket       string
	Size         int64
	ContentType  string
	ChecksumSHA256 string
	LastModified time.Time
}

// Backend abstracts MinIO / S3 / local filesystem.
type Backend interface {
	// Put stores the full body under key.  Returns size + checksum.
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) (ObjectInfo, error)

	// Get returns a ReadCloser for the object body.
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)

	// Head returns metadata without the body.
	Head(ctx context.Context, key string) (ObjectInfo, error)

	// Delete removes the object.
	Delete(ctx context.Context, key string) error

	// PresignGet returns a time-limited download URL (S3-compatible only).
	// Filesystem backend returns a file:// style path or empty string.
	PresignGet(ctx context.Context, key string, expires time.Duration) (string, time.Time, error)

	// EnsureBucket creates the bucket if it does not exist.
	EnsureBucket(ctx context.Context) error

	// Name returns a human-readable backend identifier for health checks.
	Name() string
}
