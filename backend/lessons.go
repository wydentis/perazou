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
	Parity    int    `json:"parity"`     // 0 every week, 1 odd, 2 even
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

func mondayOf(d time.Time) time.Time {
	return d.AddDate(0, 0, -(isoWeekday(d) - 1))
}

// isOddWeek reports whether date falls in an odd week, given the Monday of an odd week.
// All inputs are calendar dates (midnight UTC).
func isOddWeek(week1Monday, date time.Time) bool {
	days := int(mondayOf(date).Sub(week1Monday).Hours() / 24)
	weeks := days / 7
	if days < 0 && days%7 != 0 {
		weeks--
	}
	return ((weeks%2)+2)%2 == 0
}

// occursOn reports whether a slot is scheduled on the given date.
func (s slot) occursOn(week1Monday, date time.Time) bool {
	if s.Weekday != isoWeekday(date) {
		return false
	}
	// Dates are compared as YYYY-MM-DD strings, which sort chronologically.
	if d := date.Format(dateLayout); (s.ValidFrom != "" && d < s.ValidFrom) || (s.ValidTo != "" && d > s.ValidTo) {
		return false
	}
	switch s.Parity {
	case 1:
		return isOddWeek(week1Monday, date)
	case 2:
		return !isOddWeek(week1Monday, date)
	}
	return true
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

// markWindow is how long after a lesson starts students may mark themselves.
const markWindow = 15

// markClosesAt returns the "HH:MM" at which student marking closes for a lesson.
func markClosesAt(startTime string) string {
	m := clockMinutes(startTime) + markWindow
	if m >= 24*60 {
		m = 24*60 - 1
	}
	return time.Date(0, 1, 1, m/60, m%60, 0, 0, time.UTC).Format("15:04")
}

// lessonState describes where a lesson is relative to "now" in the group's timezone.
// Students may mark themselves during the first markWindow minutes of the lesson;
// the headman may edit any lesson that has started.
func lessonState(now time.Time, date time.Time, startTime string) (started, studentOpen bool) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if date.Before(today) {
		return true, false
	}
	if date.After(today) {
		return false, false
	}
	nowMin, start := now.Hour()*60+now.Minute(), clockMinutes(startTime)
	return nowMin >= start, nowMin >= start && nowMin < clockMinutes(markClosesAt(startTime))
}
