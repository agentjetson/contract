package registry

import (
	"sync"
	"time"
)

// Live tracks dynamically registered sources and emit debounce.
type Live struct {
	mu       sync.Mutex
	sources  map[string]time.Time // last seen
	lastEmit map[string]time.Time
	debounce time.Duration
}

func New(debounce time.Duration) *Live {
	if debounce <= 0 {
		debounce = 60 * time.Second
	}
	return &Live{
		sources:  make(map[string]time.Time),
		lastEmit: make(map[string]time.Time),
		debounce: debounce,
	}
}

// Touch records an up/heartbeat. Returns whether a profile emit should run
// (first sighting or debounce elapsed).
func (l *Live) Touch(id string, now time.Time) (shouldEmit bool) {
	if id == "" {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sources[id] = now
	prev, ok := l.lastEmit[id]
	if !ok || now.Sub(prev) >= l.debounce {
		l.lastEmit[id] = now
		return true
	}
	return false
}

// Remove clears a source on down.
func (l *Live) Remove(id string) {
	if id == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.sources, id)
	// keep lastEmit so a quick flap does not spam; expires via debounce on next up
}

// Count of currently live sources.
func (l *Live) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.sources)
}
