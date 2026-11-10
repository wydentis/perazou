package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

// userError is a refusal the caller should see as-is (bad input or a rule), as opposed to a
// server failure.
type userError struct {
	code int
	msg  string
}

func (e *userError) Error() string { return e.msg }

func refuse(code int, msg string) error { return &userError{code, msg} }

// loadLesson returns a slot of the session's group scheduled on dateStr.
func (a *app) loadLesson(ctx context.Context, s *session, slotID int64, dateStr string) (slot, time.Time, error) {
	date, err := parseDate(dateStr)
	if err != nil {
		return slot{}, date, refuse(http.StatusBadRequest, "invalid date")
	}
	sl, err := scanSlot(a.db.QueryRow(ctx, `SELECT `+slotCols+` FROM schedule_slots WHERE id=$1 AND group_id=$2`, slotID, s.Group.ID))
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !sl.occursOn(date)) {
		return slot{}, date, refuse(http.StatusNotFound, "lesson not found")
	}
	return sl, date, err
}

// applyMark sets (present/absent) or clears (none) target's mark on a lesson, enforcing the
// same rules wherever the mark comes from (the Mini App or the bot's buttons): students
// mark only themselves and only while the window is open; the headman may mark anyone in
// the group once the lesson has started or its window is open.
func (a *app) applyMark(ctx context.Context, s *session, target, slotID int64, dateStr, status string) (slot, error) {
	if target != s.UserID && s.Role != "headman" {
		return slot{}, refuse(http.StatusForbidden, "headman only")
	}
	sl, date, err := a.loadLesson(ctx, s, slotID, dateStr)
	if err != nil {
		return sl, err
	}
	var cancelled bool
	if err := a.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM lesson_cancellations WHERE slot_id=$1 AND lesson_date=$2::date)`,
		sl.ID, dateStr).Scan(&cancelled); err != nil {
		return sl, err
	}
	if cancelled {
		return sl, refuse(http.StatusConflict, "lesson is cancelled")
	}
	started, open := lessonState(s.Group.now(), date, sl.StartTime)
	if !started && !open {
		return sl, refuse(http.StatusConflict, "marking has not opened yet")
	}
	if s.Role != "headman" && !open {
		return sl, refuse(http.StatusConflict, "marking is closed for this lesson")
	}
	if status == "none" {
		_, err = a.db.Exec(ctx, `DELETE FROM attendance a USING members m
			WHERE a.slot_id=$1 AND a.lesson_date=$2::date AND a.user_id=$3 AND m.user_id=a.user_id AND m.group_id=$4`,
			sl.ID, dateStr, target, s.Group.ID)
		return sl, err
	}
	var n int64
	// Insert only for members of the caller's group.
	err = a.db.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO attendance (slot_id, lesson_date, user_id, status, marked_by)
			SELECT $1, $2::date, m.user_id, $4, $5 FROM members m WHERE m.user_id=$3 AND m.group_id=$6
			ON CONFLICT (slot_id, lesson_date, user_id) DO UPDATE SET status=$4, marked_by=$5, updated_at=now()
			RETURNING 1)
		SELECT count(*) FROM ins`, sl.ID, dateStr, target, status, s.UserID, s.Group.ID).Scan(&n)
	if err == nil && n == 0 {
		return sl, refuse(http.StatusNotFound, "member not found")
	}
	return sl, err
}
