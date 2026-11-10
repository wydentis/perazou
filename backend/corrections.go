package main

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// applyCorrection changes a student's own mark on a past lesson and logs it, in one
// transaction. A missing mark counts as absent ("auto"), so fixing it to present is the
// usual case; setting the same status again is not logged.
func (a *app) applyCorrection(ctx context.Context, s *session, sl slot, date, status string) error {
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	old := "auto"
	if err := tx.QueryRow(ctx, `SELECT status FROM attendance WHERE slot_id=$1 AND lesson_date=$2::date AND user_id=$3 FOR UPDATE`,
		sl.ID, date, s.UserID).Scan(&old); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if old == status {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO attendance (slot_id, lesson_date, user_id, status, marked_by) VALUES ($1, $2::date, $3, $4, $3)
		ON CONFLICT (slot_id, lesson_date, user_id) DO UPDATE SET status=$4, marked_by=$3, updated_at=now()`,
		sl.ID, date, s.UserID, status); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO attendance_corrections (slot_id, lesson_date, user_id, old_status, new_status)
		VALUES ($1, $2::date, $3, $4, $5)`, sl.ID, date, s.UserID, old, status); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (a *app) setCorrectionsOpen(c *gin.Context) {
	var req struct {
		Open bool `json:"open"`
	}
	if !bind(c, &req) {
		return
	}
	if _, err := a.db.Exec(c, `UPDATE groups SET corrections_open=$2 WHERE id=$1`, sess(c).Group.ID, req.Open); err != nil {
		serverErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type correctableLesson struct {
	Date      string `json:"date"`
	SlotID    int64  `json:"slot_id"`
	Subject   string `json:"subject"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Status    string `json:"status"` // present | absent | "" (open window, not marked yet)
	Auto      bool   `json:"auto"`   // absent because nothing was marked
	Corrected bool   `json:"corrected"`
}

// myCorrectableLessons lists the caller's lessons that have already started, newest first,
// from the day they joined (at most 180 days back), with their current mark.
func (a *app) myCorrectableLessons(c *gin.Context) {
	s := sess(c)
	now := s.Group.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	var joined string
	if err := a.db.QueryRow(c, `SELECT to_char(joined_at AT TIME ZONE $2, 'YYYY-MM-DD') FROM members WHERE user_id=$1`,
		s.UserID, s.Group.Timezone).Scan(&joined); err != nil {
		serverErr(c, err)
		return
	}
	from := today.AddDate(0, 0, -180)
	if j, err := parseDate(joined); err == nil && j.After(from) {
		from = j
	}
	all, err := a.slots(c, s.Group.ID)
	if err != nil {
		serverErr(c, err)
		return
	}

	key := func(slotID int64, d string) string { return strconv.FormatInt(slotID, 10) + d }
	cancelled := map[string]bool{}
	marks := map[string]string{}
	corrected := map[string]bool{}
	type q struct {
		sql  string
		args []any
		into func(slotID int64, d, v string)
	}
	queries := []q{
		{`SELECT lc.slot_id, to_char(lc.lesson_date,'YYYY-MM-DD'), '' FROM lesson_cancellations lc
			JOIN schedule_slots ss ON ss.id = lc.slot_id WHERE ss.group_id=$1 AND lc.lesson_date >= $2::date`,
			[]any{s.Group.ID, from}, func(id int64, d, _ string) { cancelled[key(id, d)] = true }},
		{`SELECT slot_id, to_char(lesson_date,'YYYY-MM-DD'), status FROM attendance WHERE user_id=$1 AND lesson_date >= $2::date`,
			[]any{s.UserID, from}, func(id int64, d, v string) { marks[key(id, d)] = v }},
		{`SELECT DISTINCT slot_id, to_char(lesson_date,'YYYY-MM-DD'), '' FROM attendance_corrections WHERE user_id=$1 AND lesson_date >= $2::date`,
			[]any{s.UserID, from}, func(id int64, d, _ string) { corrected[key(id, d)] = true }},
	}
	for _, qq := range queries {
		rows, err := a.db.Query(c, qq.sql, qq.args...)
		if err != nil {
			serverErr(c, err)
			return
		}
		for rows.Next() {
			var id int64
			var d, v string
			if err := rows.Scan(&id, &d, &v); err != nil {
				rows.Close()
				serverErr(c, err)
				return
			}
			qq.into(id, d, v)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			serverErr(c, err)
			return
		}
	}

	out := []correctableLesson{}
	for d := today; !d.Before(from); d = d.AddDate(0, 0, -1) {
		ds := d.Format(dateLayout)
		day := []correctableLesson{}
		for _, sl := range all {
			k := key(sl.ID, ds)
			if !sl.occursOn(d) || cancelled[k] {
				continue
			}
			started, open := lessonState(now, d, sl.StartTime)
			if !started {
				continue
			}
			l := correctableLesson{Date: ds, SlotID: sl.ID, Subject: sl.Subject, StartTime: sl.StartTime, EndTime: sl.EndTime,
				Status: marks[k], Corrected: corrected[k]}
			if l.Status == "" && autoAbsent(started, open, false) {
				l.Status, l.Auto = "absent", true
			}
			day = append(day, l)
		}
		sort.Slice(day, func(i, j int) bool { return day[i].StartTime > day[j].StartTime })
		out = append(out, day...)
	}
	c.JSON(http.StatusOK, gin.H{"corrections_open": s.Group.CorrectionsOpen, "lessons": out})
}

type correctionEntry struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	FullName  string    `json:"full_name"`
	Subject   string    `json:"subject"`
	Date      string    `json:"date"`
	StartTime string    `json:"start_time"`
	Old       string    `json:"old_status"`
	New       string    `json:"new_status"`
	ChangedAt time.Time `json:"changed_at"`
}

// correctionsLog shows the headman who changed their own marks, newest first.
func (a *app) correctionsLog(c *gin.Context) {
	rows, err := a.db.Query(c, `
		SELECT ac.id, ac.user_id, m.full_name, ss.subject, to_char(ac.lesson_date,'YYYY-MM-DD'),
			to_char(ss.start_time,'HH24:MI'), ac.old_status, ac.new_status, ac.changed_at
		FROM attendance_corrections ac
		JOIN schedule_slots ss ON ss.id = ac.slot_id
		JOIN members m ON m.user_id = ac.user_id
		WHERE ss.group_id = $1
		ORDER BY ac.changed_at DESC
		LIMIT 300`, sess(c).Group.ID)
	if err != nil {
		serverErr(c, err)
		return
	}
	defer rows.Close()
	out := []correctionEntry{}
	for rows.Next() {
		var e correctionEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.FullName, &e.Subject, &e.Date, &e.StartTime, &e.Old, &e.New, &e.ChangedAt); err != nil {
			serverErr(c, err)
			return
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}
