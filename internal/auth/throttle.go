package auth

import (
	"sync"
	"time"
)

// Throttle counts recent failures per key in memory. Nothing is persisted,
// so keys that contain IP addresses vanish on restart.
type Throttle struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	fails  map[string][]time.Time
}

func NewThrottle(max int, window time.Duration, now func() time.Time) *Throttle {
	return &Throttle{max: max, window: window, now: now, fails: map[string][]time.Time{}}
}

func (t *Throttle) Allowed(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.recent(key)) < t.max
}

func (t *Throttle) Fail(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.fails[key] = append(t.recent(key), t.now())
	if len(t.fails) > 10_000 { // bound memory under a flood of distinct keys
		for k := range t.fails {
			if len(t.recent(k)) == 0 {
				delete(t.fails, k)
			}
		}
	}
}

func (t *Throttle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.fails, key)
}

func (t *Throttle) recent(key string) []time.Time {
	cutoff := t.now().Add(-t.window)
	kept := t.fails[key][:0]
	for _, at := range t.fails[key] {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(t.fails, key)
		return nil
	}
	t.fails[key] = kept
	return kept
}
