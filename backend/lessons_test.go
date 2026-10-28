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
	if s, o := lessonState(now, d("2026-09-09"), "09:50"); !s || !o {
		t.Error("lesson started 10 minutes ago must be open")
	}
	if s, o := lessonState(now, d("2026-09-09"), "09:45"); !s || o {
		t.Error("marking must close exactly 15 minutes after start")
	}
	if s, o := lessonState(now, d("2026-09-09"), "09:00"); !s || o {
		t.Error("lesson started an hour ago must be closed for students")
	}
	if s, o := lessonState(now, d("2026-09-09"), "11:00"); s || o {
		t.Error("future lesson today must be closed")
	}
	if s, o := lessonState(now, d("2026-09-08"), "09:00"); !s || o {
		t.Error("past day: started but closed for students")
	}
	if s, _ := lessonState(now, d("2026-09-10"), "09:00"); s {
		t.Error("tomorrow not started")
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

func TestMarkClosesAt(t *testing.T) {
	if got := markClosesAt("08:15"); got != "08:30" {
		t.Errorf("got %s", got)
	}
	if got := markClosesAt("23:50"); got != "23:59" {
		t.Errorf("late lesson: got %s", got)
	}
}
