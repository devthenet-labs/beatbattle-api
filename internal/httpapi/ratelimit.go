package httpapi

import (
	"math"
	"sync"
	"time"
)

// voteLimit and voteWindow bound the vote rate: voteLimit votes per key per
// voteWindow, counted on a sliding window.
const (
	voteLimit  = 3
	voteWindow = 10 * time.Second
)

// voteLimiter is shared across every caller of allow; the empty key is
// requests without an X-Client-Id header.
type voteLimiter struct {
	mu   sync.Mutex
	now  func() time.Time
	hits map[string][]time.Time
}

func newVoteLimiter(now func() time.Time) *voteLimiter {
	return &voteLimiter{now: now, hits: make(map[string][]time.Time)}
}

// allow reports whether key may vote now. A refusal is not itself recorded,
// so a client stuck at the limit does not push its own window further out by
// continuing to try. On refusal, the returned int is the whole seconds until
// the oldest kept hit falls out of the window, rounded up, with a floor of 1.
func (l *voteLimiter) allow(key string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-voteWindow)
	hits := l.hits[key]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) < voteLimit {
		l.hits[key] = append(kept, now)
		return true, 0
	}

	l.hits[key] = kept
	remaining := kept[0].Add(voteWindow).Sub(now)
	seconds := int(math.Ceil(remaining.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return false, seconds
}
