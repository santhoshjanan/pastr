package auth

import (
	"sync"
	"time"
)

// maxThrottleKeys bounds memory when a flood of distinct keys arrives.
const maxThrottleKeys = 20_000

// Throttle limits attempts per key within a sliding window, in memory only.
// Nothing is persisted, so keys that contain IP addresses vanish on restart.
type Throttle struct {
	mu       sync.Mutex
	max      int
	window   time.Duration
	now      func() time.Time
	attempts map[string][]time.Time
}

func NewThrottle(max int, window time.Duration, now func() time.Time) *Throttle {
	return &Throttle{max: max, window: window, now: now, attempts: map[string][]time.Time{}}
}

// Attempt reserves a slot for key and reports whether one was free. Checking and
// recording are one step, so parallel requests cannot all slip under the limit.
func (t *Throttle) Attempt(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	recent := t.recent(key)
	if len(recent) >= t.max {
		return false
	}
	if _, known := t.attempts[key]; !known && len(t.attempts) >= maxThrottleKeys {
		t.evict()
	}
	t.attempts[key] = append(recent, t.now())
	return true
}

// Release hands back the most recent slot, for an attempt that turned out to be legitimate.
func (t *Throttle) Release(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if a := t.attempts[key]; len(a) > 1 {
		t.attempts[key] = a[:len(a)-1]
	} else {
		delete(t.attempts, key)
	}
}

func (t *Throttle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, key)
}

// recent drops attempts older than the window and returns what is left.
func (t *Throttle) recent(key string) []time.Time {
	all := t.attempts[key]
	cutoff := t.now().Add(-t.window)
	kept := all[:0]
	for _, at := range all {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	if len(kept) == 0 {
		delete(t.attempts, key)
		return nil
	}
	t.attempts[key] = kept
	return kept
}

// evict makes room when the map is full: stale keys go first, then arbitrary ones.
// Dropping a live key forgives its attempts, which is the price of a hard memory cap.
func (t *Throttle) evict() {
	for k := range t.attempts {
		t.recent(k)
	}
	for k := range t.attempts {
		if len(t.attempts) < maxThrottleKeys*9/10 {
			return
		}
		delete(t.attempts, k)
	}
}
