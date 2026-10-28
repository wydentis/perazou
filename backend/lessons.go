package main

import (
	"time"
)

type slot struct {
	ID        int64  `json:"id"`
	Weekday   int    `json:"weekday"` // 1 = Monday ... 7 = Sunday
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Subject   string `json:"subject"`
	Room      string `json:"room"`
	Teacher   string `json:"teacher"`
	ValidFrom string `json:"valid_from"` // YYYY-MM-DD or "" (no bound)
	ValidTo   string `json:"valid_to"`
}

const dateLayout = "2006-01-02"

func isoWeekday(t time.Time) int {
	if t.Weekday() == time.Sunday {
		return 7
	}
	return int(t.Weekday())
}

// occursOn reports whether a slot is scheduled on the given date (a calendar date, midnight UTC).
func (s slot) occursOn(date time.Time) bool {
	if s.Weekday != isoWeekday(date) {
		return false
	}
	// Dates are compared as YYYY-MM-DD strings, which sort chronologically.
	d := date.Format(dateLayout)
	return (s.ValidFrom == "" || d >= s.ValidFrom) && (s.ValidTo == "" || d <= s.ValidTo)
}

func parseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, s)
}

// clockMinutes parses "HH:MM" into minutes since midnight.
func clockMinutes(s string) int {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0
	}
	return t.Hour()*60 + t.Minute()
}

// Students may mark themselves from markBefore minutes before a lesson starts until
// markAfter minutes after it started.
const (
	markBefore = 15
	markAfter  = 30
)

// clockAt formats minutes since midnight as "HH:MM", clamped to the same day.
func clockAt(m int) string {
	m = min(max(m, 0), 24*60-1)
	return time.Date(0, 1, 1, m/60, m%60, 0, 0, time.UTC).Format("15:04")
}

// markOpensAt / markClosesAt return the "HH:MM" bounds of the student marking window.
func markOpensAt(startTime string) string  { return clockAt(clockMinutes(startTime) - markBefore) }
func markClosesAt(startTime string) string { return clockAt(clockMinutes(startTime) + markAfter) }

// lessonState describes where a lesson is relative to "now" in the group's timezone:
// started once its start time has passed, studentOpen while the marking window is open.
func lessonState(now time.Time, date time.Time, startTime string) (started, studentOpen bool) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if date.Before(today) {
		return true, false
	}
	if date.After(today) {
		return false, false
	}
	nowMin := now.Hour()*60 + now.Minute()
	return nowMin >= clockMinutes(startTime),
		nowMin >= clockMinutes(markOpensAt(startTime)) && nowMin < clockMinutes(markClosesAt(startTime))
}
