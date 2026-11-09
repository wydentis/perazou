package main

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := newRateLimiter(120, 120)
	l.now = func() time.Time { return now }

	for i := 0; i < 120; i++ {
		if ok, _ := l.allow(1); !ok {
			t.Fatalf("request %d of the burst refused", i+1)
		}
	}
	ok, wait := l.allow(1)
	if ok || wait <= 0 || wait > time.Second {
		t.Fatalf("burst exhausted: ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.allow(2); !ok {
		t.Fatal("another user must not be affected")
	}
	now = now.Add(time.Second) // 120/min refills 2 tokens per second
	for i := 0; i < 2; i++ {
		if ok, _ := l.allow(1); !ok {
			t.Fatalf("refilled token %d refused", i+1)
		}
	}
	if ok, _ := l.allow(1); ok {
		t.Fatal("only 2 tokens should refill per second")
	}

	now = now.Add(time.Hour)
	l.sweep(now)
	if len(l.buckets) != 0 {
		t.Errorf("idle buckets not swept: %d", len(l.buckets))
	}
}
