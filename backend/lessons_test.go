package main

import (
	"testing"
	"time"
)

func d(s string) time.Time { t, _ := parseDate(s); return t }

func TestOccursOn(t *testing.T) {
	s := slot{Weekday: 3}
	for date, want := range map[string]bool{"2026-09-09": true, "2026-09-16": true, "2026-09-17": false} {
		if got := s.occursOn(d(date)); got != want {
			t.Errorf("%s: got %v want %v", date, got, want)
		}
	}
}

func TestLessonState(t *testing.T) {
	now := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		date, start   string
		started, open bool
		why           string
	}{
		{"2026-09-09", "10:10", false, true, "10 min before start: window already open"},
		{"2026-09-09", "10:15", false, true, "window opens exactly 15 min before (inclusive)"},
		{"2026-09-09", "10:16", false, false, "16 min before start: not open yet"},
		{"2026-09-09", "09:31", true, true, "29 min after start"},
		{"2026-09-09", "09:30", true, false, "window closes exactly 30 min after"},
		{"2026-09-09", "09:00", true, false, "an hour after start"},
		{"2026-09-08", "09:00", true, false, "past day"},
		{"2026-09-10", "09:00", false, false, "tomorrow"},
	}
	for _, c := range cases {
		if s, o := lessonState(now, d(c.date), c.start); s != c.started || o != c.open {
			t.Errorf("%s: started=%v open=%v", c.why, s, o)
		}
	}
}

func TestValidRange(t *testing.T) {
	s := slot{Weekday: 1, ValidFrom: "2026-09-14", ValidTo: "2026-10-19"}
	for date, want := range map[string]bool{"2026-09-07": false, "2026-09-14": true, "2026-10-19": true, "2026-10-26": false} {
		if got := s.occursOn(d(date)); got != want {
			t.Errorf("%s: got %v want %v", date, got, want)
		}
	}
}

func TestMarkWindowBounds(t *testing.T) {
	if o, c := markOpensAt("08:15"), markClosesAt("08:15"); o != "08:00" || c != "08:45" {
		t.Errorf("08:15: %s-%s", o, c)
	}
	if o, c := markOpensAt("00:05"), markClosesAt("23:50"); o != "00:00" || c != "23:59" {
		t.Errorf("clamping: %s %s", o, c)
	}
}
