package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioBackend talks to any S3-compatible endpoint (MinIO, AWS S3, …).
type MinioBackend struct {
	client *minio.Client
	bucket string
	name   string
}

func NewMinioBackend(endpoint, accessKey, secretKey, bucket, region string, useSSL, forcePathStyle bool) (*MinioBackend, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	}
	if forcePathStyle {
		opts.BucketLookup = minio.BucketLookupPath
	}
	client, err := minio.New(endpoint, opts)
	if err != nil {
		return nil, err
	}
	name := "minio"
	if region != "" && region != "us-east-1" {
		name = "s3"
	}
	return &MinioBackend{client: client, bucket: bucket, name: name}, nil
}

func (m *MinioBackend) Name() string { return m.name }

func (m *MinioBackend) EnsureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return m.client.MakeBucket(ctx, m.bucket, minio.MakeBucketOptions{})
}

func (m *MinioBackend) Put(ctx context.Context, key, contentType string, body io.Reader, size int64) (ObjectInfo, error) {
	// Tee into a hasher so we can return a checksum without a second pass.
	h := sha256.New()
	tee := io.TeeReader(body, h)

	info, err := m.client.PutObject(ctx, m.bucket, key, tee, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		Key:            key,
		Bucket:         m.bucket,
		Size:           info.Size,
		ContentType:    contentType,
		ChecksumSHA256: hex.EncodeToString(h.Sum(nil)),
		LastModified:   time.Now().UTC(),
	}, nil
}

func (m *MinioBackend) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	st, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, ObjectInfo{}, err
	}
	return obj, ObjectInfo{
		Key:          key,
		Bucket:       m.bucket,
		Size:         st.Size,
		ContentType:  st.ContentType,
		LastModified: st.LastModified.UTC(),
	}, nil
}

func (m *MinioBackend) Head(ctx context.Context, key string) (ObjectInfo, error) {
	st, err := m.client.StatObject(ctx, m.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		Key:          key,
		Bucket:       m.bucket,
		Size:         st.Size,
		ContentType:  st.ContentType,
		LastModified: st.LastModified.UTC(),
	}, nil
}

func (m *MinioBackend) Delete(ctx context.Context, key string) error {
	return m.client.RemoveObject(ctx, m.bucket, key, minio.RemoveObjectOptions{})
}

func (m *MinioBackend) PresignGet(ctx context.Context, key string, expires time.Duration) (string, time.Time, error) {
	u, err := m.client.PresignedGetObject(ctx, m.bucket, key, expires, url.Values{})
	if err != nil {
		return "", time.Time{}, err
	}
	return u.String(), time.Now().UTC().Add(expires), nil
}
