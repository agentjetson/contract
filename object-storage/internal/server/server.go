package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/agentjetson/object-storage/internal/metadata"
	"github.com/agentjetson/object-storage/internal/storage"
)

// NOTE: Generated stubs live under gen/ once `buf generate` / protoc is run.
// For the scaffold we define a minimal hand-written interface that matches the
// proto so the service is compile-able without code-gen in DEMO_MODE.

// ---------------------------------------------------------------------------
// Hand-written message types mirroring storage.v1 (keeps the scaffold green
// without requiring buf/protoc in the download zip).  Replace with generated
// code when integrating into the Buf workspace.
// ---------------------------------------------------------------------------

type ObjectKind int32

const (
	ObjectKindUnspecified      ObjectKind = 0
	ObjectKindAudioTranscript  ObjectKind = 1
	ObjectKindCameraRecording  ObjectKind = 2
	ObjectKindFrameSnapshot    ObjectKind = 3
	ObjectKindCrop             ObjectKind = 4
	ObjectKindAnnotatedClip    ObjectKind = 5
	ObjectKindOther            ObjectKind = 99
)

func kindName(k ObjectKind) string {
	switch k {
	case ObjectKindAudioTranscript:
		return "audio_transcript"
	case ObjectKindCameraRecording:
		return "camera_recording"
	case ObjectKindFrameSnapshot:
		return "frame_snapshot"
	case ObjectKindCrop:
		return "crop"
	case ObjectKindAnnotatedClip:
		return "annotated_clip"
	case ObjectKindOther:
		return "other"
	default:
		return "unspecified"
	}
}

func kindPrefix(k ObjectKind) string {
	return kindName(k)
}

// ObjectMeta mirrors storage.v1.ObjectMeta.
type ObjectMeta struct {
	ObjectID       string
	Kind           ObjectKind
	Source         string
	Timestamp      time.Time
	FrameID        int64
	TrackID        int32
	ContentType    string
	SizeBytes      int64
	ChecksumSHA256 string
	Labels         map[string]string
	StorageKey     string
	Bucket         string
	CreatedAt      time.Time
}

type DomainError struct {
	Code    string
	Message string
}

type PutObjectRequest struct {
	Kind        ObjectKind
	Source      string
	Timestamp   time.Time
	FrameID     int64
	TrackID     int32
	ContentType string
	Data        []byte
	Labels      map[string]string
	ObjectID    string
}

type PutObjectResponse struct {
	Accepted bool
	Meta     *ObjectMeta
	Error    *DomainError
}

type GetObjectMetaRequest struct {
	ObjectID   string
	StorageKey string
}

type GetObjectMetaResponse struct {
	Meta  *ObjectMeta
	Error *DomainError
}

type ListObjectsRequest struct {
	Source    string
	Kind      ObjectKind
	StartTime time.Time
	EndTime   time.Time
	Limit     int32
	PageToken string
}

type ListObjectsResponse struct {
	Objects       []*ObjectMeta
	NextPageToken string
	Error         *DomainError
}

type DeleteObjectRequest struct {
	ObjectID   string
	StorageKey string
	HardDelete bool
}

type DeleteObjectResponse struct {
	Deleted bool
	Error   *DomainError
}

type HealthRequest struct{}

type HealthResponse struct {
	Ok      bool
	Backend string
	Detail  string
}

// Service implements the ObjectStorage RPCs.
type Service struct {
	backend storage.Backend
	meta    metadata.Store
	bucket  string
}

func New(backend storage.Backend, meta metadata.Store, bucket string) *Service {
	return &Service{backend: backend, meta: meta, bucket: bucket}
}

// PutObject stores a complete blob and records metadata.
func (s *Service) PutObject(ctx context.Context, req *PutObjectRequest) (*PutObjectResponse, error) {
	if req == nil || len(req.Data) == 0 {
		return &PutObjectResponse{
			Accepted: false,
			Error:    &DomainError{Code: "INVALID_ARGUMENT", Message: "empty payload"},
		}, nil
	}
	if req.Source == "" {
		return &PutObjectResponse{
			Accepted: false,
			Error:    &DomainError{Code: "INVALID_ARGUMENT", Message: "source is required"},
		}, nil
	}

	objectID := req.ObjectID
	if objectID == "" {
		objectID = uuid.NewString()
	}
	ts := req.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	contentType := req.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	key := buildKey(req.Kind, req.Source, ts, objectID, contentType)
	info, err := s.backend.Put(ctx, key, contentType, bytes.NewReader(req.Data), int64(len(req.Data)))
	if err != nil {
		log.Printf("put failed: %v", err)
		return &PutObjectResponse{
			Accepted: false,
			Error:    &DomainError{Code: "STORAGE_ERROR", Message: err.Error()},
		}, nil
	}

	rec := metadata.Record{
		ObjectID:       objectID,
		Kind:           kindName(req.Kind),
		Source:         req.Source,
		EventTS:        ts,
		FrameID:        req.FrameID,
		TrackID:        req.TrackID,
		ContentType:    contentType,
		SizeBytes:      info.Size,
		ChecksumSHA256: info.ChecksumSHA256,
		StorageKey:     key,
		Bucket:         s.bucket,
		Labels:         req.Labels,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.meta.Insert(ctx, rec); err != nil {
		log.Printf("metadata insert failed (object already stored): %v", err)
	}

	return &PutObjectResponse{
		Accepted: true,
		Meta:     recordToMeta(rec, req.Kind),
	}, nil
}

// GetObjectMeta returns metadata only.
func (s *Service) GetObjectMeta(ctx context.Context, req *GetObjectMetaRequest) (*GetObjectMetaResponse, error) {
	if req.ObjectID == "" {
		return &GetObjectMetaResponse{
			Error: &DomainError{Code: "INVALID_ARGUMENT", Message: "object_id required"},
		}, nil
	}
	rec, err := s.meta.Get(ctx, req.ObjectID)
	if err != nil {
		return &GetObjectMetaResponse{
			Error: &DomainError{Code: "NOT_FOUND", Message: err.Error()},
		}, nil
	}
	return &GetObjectMetaResponse{Meta: recordToMeta(rec, ObjectKindUnspecified)}, nil
}

// ListObjects filters by source / kind / time window.
func (s *Service) ListObjects(ctx context.Context, req *ListObjectsRequest) (*ListObjectsResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	kindFilter := ""
	if req.Kind != ObjectKindUnspecified {
		kindFilter = kindName(req.Kind)
	}
	recs, err := s.meta.List(ctx, req.Source, kindFilter, req.StartTime, req.EndTime, limit)
	if err != nil {
		return &ListObjectsResponse{
			Error: &DomainError{Code: "INTERNAL", Message: err.Error()},
		}, nil
	}
	out := make([]*ObjectMeta, 0, len(recs))
	for _, r := range recs {
		out = append(out, recordToMeta(r, ObjectKindUnspecified))
	}
	return &ListObjectsResponse{Objects: out}, nil
}

// DeleteObject soft-deletes by default; hard-delete removes the blob.
func (s *Service) DeleteObject(ctx context.Context, req *DeleteObjectRequest) (*DeleteObjectResponse, error) {
	if req.ObjectID == "" {
		return &DeleteObjectResponse{
			Error: &DomainError{Code: "INVALID_ARGUMENT", Message: "object_id required"},
		}, nil
	}
	rec, err := s.meta.Get(ctx, req.ObjectID)
	if err != nil {
		return &DeleteObjectResponse{
			Error: &DomainError{Code: "NOT_FOUND", Message: err.Error()},
		}, nil
	}
	if req.HardDelete {
		if err := s.backend.Delete(ctx, rec.StorageKey); err != nil {
			return &DeleteObjectResponse{
				Error: &DomainError{Code: "STORAGE_ERROR", Message: err.Error()},
			}, nil
		}
	}
	_ = s.meta.MarkDeleted(ctx, req.ObjectID)
	return &DeleteObjectResponse{Deleted: true}, nil
}

// Health reports backend readiness.
func (s *Service) Health(ctx context.Context, _ *HealthRequest) (*HealthResponse, error) {
	return &HealthResponse{
		Ok:      true,
		Backend: s.backend.Name(),
		Detail:  fmt.Sprintf("bucket=%s", s.bucket),
	}, nil
}

// GetObjectStream opens the backend object for streaming (used by gRPC handler).
func (s *Service) GetObjectStream(ctx context.Context, objectID string) (io.ReadCloser, *ObjectMeta, error) {
	rec, err := s.meta.Get(ctx, objectID)
	if err != nil {
		return nil, nil, status.Errorf(codes.NotFound, "%v", err)
	}
	rc, _, err := s.backend.Get(ctx, rec.StorageKey)
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "backend get: %v", err)
	}
	return rc, recordToMeta(rec, ObjectKindUnspecified), nil
}

// Presign returns a temporary download URL.
func (s *Service) Presign(ctx context.Context, objectID string, expiresSec int32) (string, time.Time, error) {
	rec, err := s.meta.Get(ctx, objectID)
	if err != nil {
		return "", time.Time{}, err
	}
	if expiresSec <= 0 {
		expiresSec = 3600
	}
	return s.backend.PresignGet(ctx, rec.StorageKey, time.Duration(expiresSec)*time.Second)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func buildKey(kind ObjectKind, source string, ts time.Time, objectID, contentType string) string {
	ext := extensionFor(contentType)
	safeSource := strings.ReplaceAll(source, "/", "_")
	return path.Join(
		kindPrefix(kind),
		safeSource,
		ts.UTC().Format("2006/01/02"),
		objectID+ext,
	)
}

func extensionFor(ct string) string {
	switch {
	case strings.Contains(ct, "mp4"):
		return ".mp4"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "wav"):
		return ".wav"
	case strings.Contains(ct, "ogg"):
		return ".ogg"
	case strings.Contains(ct, "webm"):
		return ".webm"
	case strings.Contains(ct, "json"):
		return ".json"
	default:
		return ".bin"
	}
}

func recordToMeta(r metadata.Record, kindHint ObjectKind) *ObjectMeta {
	k := kindHint
	if k == ObjectKindUnspecified {
		switch r.Kind {
		case "audio_transcript":
			k = ObjectKindAudioTranscript
		case "camera_recording":
			k = ObjectKindCameraRecording
		case "frame_snapshot":
			k = ObjectKindFrameSnapshot
		case "crop":
			k = ObjectKindCrop
		case "annotated_clip":
			k = ObjectKindAnnotatedClip
		case "other":
			k = ObjectKindOther
		}
	}
	return &ObjectMeta{
		ObjectID:       r.ObjectID,
		Kind:           k,
		Source:         r.Source,
		Timestamp:      r.EventTS,
		FrameID:        r.FrameID,
		TrackID:        r.TrackID,
		ContentType:    r.ContentType,
		SizeBytes:      r.SizeBytes,
		ChecksumSHA256: r.ChecksumSHA256,
		Labels:         r.Labels,
		StorageKey:     r.StorageKey,
		Bucket:         r.Bucket,
		CreatedAt:      r.CreatedAt,
	}
}

// Timestamp helpers for callers that still deal with protobuf timestamps.
func PBTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
