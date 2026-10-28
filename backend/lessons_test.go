package main

import (
	"testing"
	"time"
)

func d(s string) time.Time { t, _ := parseDate(s); return t }

func TestParity(t *testing.T) {
	w1 := d("2026-09-07") // Monday of odd week
	cases := map[string]bool{
		"2026-09-07": true, "2026-09-13": true,
		"2026-09-14": false, "2026-09-20": false,
		"2026-09-21": true,
		"2026-08-31": false, // week before an odd week is even
		"2026-08-24": true,
	}
	for date, odd := range cases {
		if got := isOddWeek(w1, d(date)); got != odd {
			t.Errorf("%s: odd=%v want %v", date, got, odd)
		}
	}
}

func TestOccursOn(t *testing.T) {
	w1 := d("2026-09-07")
	s := slot{Weekday: 3, Parity: 2}
	if s.occursOn(w1, d("2026-09-09")) {
		t.Error("even-week slot should not occur in odd week")
	}
	if !s.occursOn(w1, d("2026-09-16")) {
		t.Error("even-week slot should occur in even week")
	}
	if s.occursOn(w1, d("2026-09-17")) {
		t.Error("wrong weekday")
	}
}

func TestLessonState(t *testing.T) {
	now := time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)
	if s, o := lessonState(now, d("2026-09-09"), "09:00"); !s || !o {
		t.Error("started lesson today must be open")
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
