package metadata

import (
	"context"
	"sync"
	"time"
)

// Record is the durable metadata row written after every successful Put.
type Record struct {
	ObjectID       string
	Kind           string
	Source         string
	EventTS        time.Time
	FrameID        int64
	TrackID        int32
	ContentType    string
	SizeBytes      int64
	ChecksumSHA256 string
	StorageKey     string
	Bucket         string
	Labels         map[string]string
	CreatedAt      time.Time
	Deleted        bool
}

// Store persists ObjectMeta for ClickHouse correlation and ListObjects.
type Store interface {
	Insert(ctx context.Context, r Record) error
	Get(ctx context.Context, objectID string) (Record, error)
	List(ctx context.Context, source, kind string, start, end time.Time, limit int) ([]Record, error)
	MarkDeleted(ctx context.Context, objectID string) error
}

// MemoryStore is used in DEMO_MODE and unit tests.
type MemoryStore struct {
	mu   sync.RWMutex
	byID map[string]Record
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byID: make(map[string]Record)}
}

func (m *MemoryStore) Insert(ctx context.Context, r Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[r.ObjectID] = r
	return nil
}

func (m *MemoryStore) Get(ctx context.Context, objectID string) (Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.byID[objectID]
	if !ok || r.Deleted {
		return Record{}, ErrNotFound
	}
	return r, nil
}

func (m *MemoryStore) List(ctx context.Context, source, kind string, start, end time.Time, limit int) ([]Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []Record
	for _, r := range m.byID {
		if r.Deleted {
			continue
		}
		if source != "" && r.Source != source {
			continue
		}
		if kind != "" && kind != "OBJECT_KIND_UNSPECIFIED" && r.Kind != kind {
			continue
		}
		if !start.IsZero() && r.EventTS.Before(start) {
			continue
		}
		if !end.IsZero() && r.EventTS.After(end) {
			continue
		}
		out = append(out, r)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (m *MemoryStore) MarkDeleted(ctx context.Context, objectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.byID[objectID]
	if !ok {
		return ErrNotFound
	}
	r.Deleted = true
	m.byID[objectID] = r
	return nil
}

// ErrNotFound is returned when an object_id is unknown.
var ErrNotFound = errNotFound{}

type errNotFound struct{}

func (errNotFound) Error() string { return "object not found" }
