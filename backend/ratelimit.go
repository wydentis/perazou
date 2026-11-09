package main

import (
	"math"
	"sync"
	"time"
)

// rateLimiter is a per-user token bucket: each user may burst up to `burst` requests and
// then sustain `perSec`. It is in-memory, so with N replicas the effective limit is up to
// N times higher, which is fine for its purpose of stopping scripted floods.
type rateLimiter struct {
	mu      sync.Mutex
	burst   float64
	perSec  float64
	buckets map[int64]*bucket
	calls   int
	now     func() time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(perMinute, burst int) *rateLimiter {
	return &rateLimiter{burst: float64(burst), perSec: float64(perMinute) / 60, buckets: map[int64]*bucket{}, now: time.Now}
}

// allow takes a token for the user. When none is left it reports how long until one is.
func (l *rateLimiter) allow(userID int64) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.calls++
	if l.calls%10000 == 0 {
		l.sweep(now)
	}
	b := l.buckets[userID]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[userID] = b
	}
	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.perSec)
	b.last = now
	if b.tokens < 1 {
		return false, time.Duration((1 - b.tokens) / l.perSec * float64(time.Second))
	}
	b.tokens--
	return true, 0
}

// sweep drops buckets that have refilled completely, keeping memory bounded.
func (l *rateLimiter) sweep(now time.Time) {
	full := time.Duration(l.burst / l.perSec * float64(time.Second))
	for id, b := range l.buckets {
		if now.Sub(b.last) > full {
			delete(l.buckets, id)
		}
	}
}
