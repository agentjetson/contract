package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// FilesystemBackend stores objects under a local root directory.
// Suitable for DEMO_MODE and single-node development.
type FilesystemBackend struct {
	root   string
	bucket string
}

func NewFilesystemBackend(root, bucket string) *FilesystemBackend {
	return &FilesystemBackend{root: root, bucket: bucket}
}

func (f *FilesystemBackend) Name() string { return "filesystem" }

func (f *FilesystemBackend) abs(key string) string {
	return filepath.Join(f.root, f.bucket, key)
}

func (f *FilesystemBackend) EnsureBucket(ctx context.Context) error {
	return os.MkdirAll(filepath.Join(f.root, f.bucket), 0o755)
}

func (f *FilesystemBackend) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) (ObjectInfo, error) {
	path := f.abs(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ObjectInfo{}, err
	}
	tmp := path + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return ObjectInfo{}, err
	}
	h := sha256.New()
	w := io.MultiWriter(out, h)
	n, err := io.Copy(w, body)
	if err != nil {
		out.Close()
		os.Remove(tmp)
		return ObjectInfo{}, err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return ObjectInfo{}, err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return ObjectInfo{}, err
	}
	// Write a sidecar content-type file for Head().
	_ = os.WriteFile(path+".ct", []byte(contentType), 0o644)
	return ObjectInfo{
		Key:            key,
		Bucket:         f.bucket,
		Size:           n,
		ContentType:    contentType,
		ChecksumSHA256: hex.EncodeToString(h.Sum(nil)),
		LastModified:   time.Now().UTC(),
	}, nil
}

func (f *FilesystemBackend) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	info, err := f.Head(ctx, key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	r, err := os.Open(f.abs(key))
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	return r, info, nil
}

func (f *FilesystemBackend) Head(ctx context.Context, key string) (ObjectInfo, error) {
	path := f.abs(key)
	st, err := os.Stat(path)
	if err != nil {
		return ObjectInfo{}, err
	}
	ct, _ := os.ReadFile(path + ".ct")
	contentType := string(ct)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return ObjectInfo{
		Key:          key,
		Bucket:       f.bucket,
		Size:         st.Size(),
		ContentType:  contentType,
		LastModified: st.ModTime().UTC(),
	}, nil
}

func (f *FilesystemBackend) Delete(ctx context.Context, key string) error {
	path := f.abs(key)
	_ = os.Remove(path + ".ct")
	return os.Remove(path)
}

func (f *FilesystemBackend) PresignGet(ctx context.Context, key string, expires time.Duration) (string, time.Time, error) {
	path := f.abs(key)
	if _, err := os.Stat(path); err != nil {
		return "", time.Time{}, err
	}
	// Local path reference — callers that need HTTP should use GetObject stream.
	return fmt.Sprintf("file://%s", path), time.Now().UTC().Add(expires), nil
}
