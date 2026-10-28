package main

import (
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

var clockRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

type slotReq struct {
	Weekday   int    `json:"weekday"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Subject   string `json:"subject"`
	Room      string `json:"room"`
	Teacher   string `json:"teacher"`
	Parity    int    `json:"parity"`
	ValidFrom string `json:"valid_from"`
	ValidTo   string `json:"valid_to"`
}

func validDateOrEmpty(s string) bool {
	if s == "" {
		return true
	}
	_, err := parseDate(s)
	return err == nil
}

func (r *slotReq) validate() bool {
	r.Subject, r.Room, r.Teacher = cleanName(r.Subject, 100), cleanName(r.Room, 60), cleanName(r.Teacher, 120)
	return r.Subject != "" && r.Weekday >= 1 && r.Weekday <= 7 && r.Parity >= 0 && r.Parity <= 2 &&
		clockRe.MatchString(r.StartTime) && clockRe.MatchString(r.EndTime) && r.EndTime > r.StartTime &&
		validDateOrEmpty(r.ValidFrom) && validDateOrEmpty(r.ValidTo) &&
		(r.ValidFrom == "" || r.ValidTo == "" || r.ValidTo >= r.ValidFrom)
}

const slotCols = `id, weekday, to_char(start_time,'HH24:MI'), to_char(end_time,'HH24:MI'), subject, room, teacher, parity,
	COALESCE(to_char(valid_from,'YYYY-MM-DD'),''), COALESCE(to_char(valid_to,'YYYY-MM-DD'),'')`

func scanSlot(row pgx.Row) (slot, error) {
	var s slot
	err := row.Scan(&s.ID, &s.Weekday, &s.StartTime, &s.EndTime, &s.Subject, &s.Room, &s.Teacher, &s.Parity, &s.ValidFrom, &s.ValidTo)
	return s, err
}

func (a *app) slots(c *gin.Context, groupID int64) ([]slot, error) {
	rows, err := a.db.Query(c, `SELECT `+slotCols+` FROM schedule_slots WHERE group_id=$1 ORDER BY weekday, start_time, id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []slot{}
	for rows.Next() {
		s, err := scanSlot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (a *app) listSlots(c *gin.Context) {
	out, err := a.slots(c, sess(c).Group.ID)
	if err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *app) createSlot(c *gin.Context) {
	var req slotReq
	if !bind(c, &req) {
		return
	}
	if !req.validate() {
		fail(c, http.StatusBadRequest, "invalid slot")
		return
	}
	s, err := scanSlot(a.db.QueryRow(c, `
		INSERT INTO schedule_slots (group_id, weekday, start_time, end_time, subject, room, teacher, parity, valid_from, valid_to)
		VALUES ($1,$2,$3::time,$4::time,$5,$6,$7,$8,NULLIF($9,'')::date,NULLIF($10,'')::date) RETURNING `+slotCols,
		sess(c).Group.ID, req.Weekday, req.StartTime, req.EndTime, req.Subject, req.Room, req.Teacher, req.Parity, req.ValidFrom, req.ValidTo))
	if err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, s)
}

func (a *app) updateSlot(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	var req slotReq
	if err != nil || !bind(c, &req) {
		if err != nil {
			fail(c, http.StatusBadRequest, "bad id")
		}
		return
	}
	if !req.validate() {
		fail(c, http.StatusBadRequest, "invalid slot")
		return
	}
	s, err := scanSlot(a.db.QueryRow(c, `
		UPDATE schedule_slots SET weekday=$3, start_time=$4::time, end_time=$5::time, subject=$6, room=$7, teacher=$8, parity=$9,
			valid_from=NULLIF($10,'')::date, valid_to=NULLIF($11,'')::date
		WHERE id=$1 AND group_id=$2 RETURNING `+slotCols,
		id, sess(c).Group.ID, req.Weekday, req.StartTime, req.EndTime, req.Subject, req.Room, req.Teacher, req.Parity, req.ValidFrom, req.ValidTo))
	if err == pgx.ErrNoRows {
		fail(c, http.StatusNotFound, "slot not found")
		return
	} else if err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

// deleteSlot also removes the slot's attendance history (ON DELETE CASCADE).
func (a *app) deleteSlot(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "bad id")
		return
	}
	tag, err := a.db.Exec(c, `DELETE FROM schedule_slots WHERE id=$1 AND group_id=$2`, id, sess(c).Group.ID)
	if err != nil {
		serverErr(c, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(c, http.StatusNotFound, "slot not found")
		return
	}
	c.Status(http.StatusNoContent)
}

// findLesson loads a slot of the caller's group and checks it is scheduled on date.
func (a *app) findLesson(c *gin.Context, s *session, slotID int64, dateStr string) (slot, time.Time, bool) {
	date, err := parseDate(dateStr)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid date")
		return slot{}, date, false
	}
	sl, err := scanSlot(a.db.QueryRow(c, `SELECT `+slotCols+` FROM schedule_slots WHERE id=$1 AND group_id=$2`, slotID, s.Group.ID))
	if err == pgx.ErrNoRows || (err == nil && !sl.occursOn(s.Group.Week1Monday, date)) {
		fail(c, http.StatusNotFound, "lesson not found")
		return slot{}, date, false
	} else if err != nil {
		serverErr(c, err)
		return slot{}, date, false
	}
	return sl, date, true
}

type lesson struct {
	slot
	Date      string `json:"date"`
	Cancelled bool   `json:"cancelled"`
	Started   bool   `json:"started"`
	// Open: the caller may change their own mark right now.
	Open     bool   `json:"open"`
	ClosesAt string `json:"closes_at"` // when student marking closes ("HH:MM")
	MyState  string `json:"my_status"` // present | absent | "" (not marked)
	MyAuto   bool   `json:"my_auto"`   // my "absent" was set automatically
	Present  int    `json:"present"`
	Absent   int    `json:"absent"` // includes AutoAbsent
	// AutoAbsent counts members who did not mark themselves before the window closed.
	AutoAbsent int `json:"auto_absent"`
	Total      int `json:"total"`
}

// autoAbsent reports whether a missing mark counts as "absent": the student window has
// closed on a lesson that took place. Nothing is written; an explicit mark always wins.
func autoAbsent(started, open, cancelled bool) bool {
	return started && !open && !cancelled
}

func (a *app) listLessons(c *gin.Context) {
	s := sess(c)
	dateStr := c.Query("date")
	if dateStr == "" {
		dateStr = s.Group.now().Format(dateLayout)
	}
	date, err := parseDate(dateStr)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid date")
		return
	}
	all, err := a.slots(c, s.Group.ID)
	if err != nil {
		serverErr(c, err)
		return
	}
	// Members who joined after this date were not expected at its lessons.
	var total int
	var meExpected bool
	if err := a.db.QueryRow(c, `SELECT count(*), COALESCE(bool_or(user_id=$4), false) FROM members
		WHERE group_id=$1 AND (joined_at AT TIME ZONE $3)::date <= $2::date`,
		s.Group.ID, dateStr, s.Group.Timezone, s.UserID).Scan(&total, &meExpected); err != nil {
		serverErr(c, err)
		return
	}
	type agg struct {
		present, absent int
		mine            string
	}
	aggs := map[int64]*agg{}
	rows, err := a.db.Query(c, `
		SELECT a.slot_id, a.user_id, a.status FROM attendance a
		JOIN schedule_slots ss ON ss.id = a.slot_id
		WHERE ss.group_id=$1 AND a.lesson_date=$2::date`, s.Group.ID, dateStr)
	if err != nil {
		serverErr(c, err)
		return
	}
	for rows.Next() {
		var slotID, uid int64
		var st string
		if err := rows.Scan(&slotID, &uid, &st); err != nil {
			rows.Close()
			serverErr(c, err)
			return
		}
		g := aggs[slotID]
		if g == nil {
			g = &agg{}
			aggs[slotID] = g
		}
		if st == "present" {
			g.present++
		} else {
			g.absent++
		}
		if uid == s.UserID {
			g.mine = st
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		serverErr(c, err)
		return
	}
	cancelled := map[int64]bool{}
	crows, err := a.db.Query(c, `
		SELECT lc.slot_id FROM lesson_cancellations lc JOIN schedule_slots ss ON ss.id = lc.slot_id
		WHERE ss.group_id=$1 AND lc.lesson_date=$2::date`, s.Group.ID, dateStr)
	if err != nil {
		serverErr(c, err)
		return
	}
	for crows.Next() {
		var id int64
		if err := crows.Scan(&id); err != nil {
			crows.Close()
			serverErr(c, err)
			return
		}
		cancelled[id] = true
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		serverErr(c, err)
		return
	}

	now := s.Group.now()
	out := []lesson{}
	for _, sl := range all {
		if !sl.occursOn(s.Group.Week1Monday, date) {
			continue
		}
		started, open := lessonState(now, date, sl.StartTime)
		l := lesson{slot: sl, Date: dateStr, Cancelled: cancelled[sl.ID], Started: started, Total: total, ClosesAt: markClosesAt(sl.StartTime)}
		l.Open = open && !l.Cancelled
		if g := aggs[sl.ID]; g != nil {
			l.Present, l.Absent, l.MyState = g.present, g.absent, g.mine
		}
		if autoAbsent(started, open, l.Cancelled) {
			l.AutoAbsent = max(0, total-l.Present-l.Absent)
			l.Absent += l.AutoAbsent
			if l.MyState == "" && meExpected {
				l.MyState, l.MyAuto = "absent", true
			}
		}
		out = append(out, l)
	}
	c.JSON(http.StatusOK, out)
}

func (a *app) lessonAttendance(c *gin.Context) {
	s := sess(c)
	slotID, err := strconv.ParseInt(c.Query("slot_id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "bad slot_id")
		return
	}
	sl, date, ok := a.findLesson(c, s, slotID, c.Query("date"))
	if !ok {
		return
	}
	var isCancelled bool
	if err := a.db.QueryRow(c, `SELECT EXISTS (SELECT 1 FROM lesson_cancellations WHERE slot_id=$1 AND lesson_date=$2::date)`,
		slotID, c.Query("date")).Scan(&isCancelled); err != nil {
		serverErr(c, err)
		return
	}
	started, open := lessonState(s.Group.now(), date, sl.StartTime)
	auto := autoAbsent(started, open, isCancelled)
	rows, err := a.db.Query(c, `
		SELECT m.user_id, m.full_name, COALESCE(a.status, ''), (m.joined_at AT TIME ZONE $4)::date <= $3::date
		FROM members m
		LEFT JOIN attendance a ON a.user_id = m.user_id AND a.slot_id=$2 AND a.lesson_date=$3::date
		WHERE m.group_id=$1 ORDER BY m.full_name`, s.Group.ID, slotID, c.Query("date"), s.Group.Timezone)
	if err != nil {
		serverErr(c, err)
		return
	}
	defer rows.Close()
	type row struct {
		UserID   int64  `json:"user_id"`
		FullName string `json:"full_name"`
		Status   string `json:"status"`
		Auto     bool   `json:"auto"`
	}
	out := []row{}
	for rows.Next() {
		var r row
		var expected bool
		if err := rows.Scan(&r.UserID, &r.FullName, &r.Status, &expected); err != nil {
			serverErr(c, err)
			return
		}
		if r.Status == "" && auto && expected {
			r.Status, r.Auto = "absent", true
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *app) cancelLesson(c *gin.Context) {
	s := sess(c)
	var req struct {
		SlotID    int64  `json:"slot_id"`
		Date      string `json:"date"`
		Cancelled bool   `json:"cancelled"`
	}
	if !bind(c, &req) {
		return
	}
	if _, _, ok := a.findLesson(c, s, req.SlotID, req.Date); !ok {
		return
	}
	var err error
	if req.Cancelled {
		_, err = a.db.Exec(c, `INSERT INTO lesson_cancellations (slot_id, lesson_date) VALUES ($1,$2::date) ON CONFLICT DO NOTHING`, req.SlotID, req.Date)
	} else {
		_, err = a.db.Exec(c, `DELETE FROM lesson_cancellations WHERE slot_id=$1 AND lesson_date=$2::date`, req.SlotID, req.Date)
	}
	if err != nil {
		serverErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// markAttendance: students set their own mark while the lesson is open; the headman can
// set or clear anyone's mark for any lesson that has started.
func (a *app) markAttendance(c *gin.Context) {
	s := sess(c)
	var req struct {
		SlotID int64  `json:"slot_id"`
		Date   string `json:"date"`
		Status string `json:"status"` // present | absent | none
		UserID *int64 `json:"user_id"`
	}
	if !bind(c, &req) {
		return
	}
	if req.Status != "present" && req.Status != "absent" && req.Status != "none" {
		fail(c, http.StatusBadRequest, "invalid status")
		return
	}
	target := s.UserID
	if req.UserID != nil {
		target = *req.UserID
	}
	if target != s.UserID && s.Role != "headman" {
		fail(c, http.StatusForbidden, "headman only")
		return
	}
	sl, date, ok := a.findLesson(c, s, req.SlotID, req.Date)
	if !ok {
		return
	}
	var isCancelled bool
	if err := a.db.QueryRow(c, `SELECT EXISTS (SELECT 1 FROM lesson_cancellations WHERE slot_id=$1 AND lesson_date=$2::date)`, sl.ID, req.Date).Scan(&isCancelled); err != nil {
		serverErr(c, err)
		return
	}
	if isCancelled {
		fail(c, http.StatusConflict, "lesson is cancelled")
		return
	}
	started, open := lessonState(s.Group.now(), date, sl.StartTime)
	if !started {
		fail(c, http.StatusConflict, "lesson has not started yet")
		return
	}
	if s.Role != "headman" && !open {
		fail(c, http.StatusConflict, "marking is closed for this lesson")
		return
	}
	var err error
	if req.Status == "none" {
		_, err = a.db.Exec(c, `DELETE FROM attendance a USING members m
			WHERE a.slot_id=$1 AND a.lesson_date=$2::date AND a.user_id=$3 AND m.user_id=a.user_id AND m.group_id=$4`,
			sl.ID, req.Date, target, s.Group.ID)
	} else {
		var tag int64
		// Insert only for members of the caller's group.
		err = a.db.QueryRow(c, `
			WITH ins AS (
				INSERT INTO attendance (slot_id, lesson_date, user_id, status, marked_by)
				SELECT $1, $2::date, m.user_id, $4, $5 FROM members m WHERE m.user_id=$3 AND m.group_id=$6
				ON CONFLICT (slot_id, lesson_date, user_id) DO UPDATE SET status=$4, marked_by=$5, updated_at=now()
				RETURNING 1)
			SELECT count(*) FROM ins`, sl.ID, req.Date, target, req.Status, s.UserID, s.Group.ID).Scan(&tag)
		if err == nil && tag == 0 {
			fail(c, http.StatusNotFound, "member not found")
			return
		}
	}
	if err != nil {
		serverErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

type statRow struct {
	UserID   int64  `json:"user_id"`
	FullName string `json:"full_name"`
	Present  int    `json:"present"`
	Absent   int    `json:"absent"`
	Unmarked int    `json:"unmarked"`
	Total    int    `json:"total"`
}

// stats counts every non-cancelled, already-started lesson in [from, to] against each member.
// Students only get their own row.
func (a *app) stats(c *gin.Context) {
	s := sess(c)
	now := s.Group.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	to, err1 := parseDate(c.DefaultQuery("to", today.Format(dateLayout)))
	from, err2 := parseDate(c.DefaultQuery("from", today.AddDate(0, 0, -29).Format(dateLayout)))
	if err1 != nil || err2 != nil || to.Before(from) || to.Sub(from) > 366*24*time.Hour {
		fail(c, http.StatusBadRequest, "invalid range")
		return
	}
	all, err := a.slots(c, s.Group.ID)
	if err != nil {
		serverErr(c, err)
		return
	}
	cancelled := map[string]bool{}
	crows, err := a.db.Query(c, `
		SELECT lc.slot_id, to_char(lc.lesson_date,'YYYY-MM-DD') FROM lesson_cancellations lc
		JOIN schedule_slots ss ON ss.id = lc.slot_id
		WHERE ss.group_id=$1 AND lc.lesson_date BETWEEN $2::date AND $3::date`, s.Group.ID, from, to)
	if err != nil {
		serverErr(c, err)
		return
	}
	for crows.Next() {
		var id int64
		var d string
		if err := crows.Scan(&id, &d); err != nil {
			crows.Close()
			serverErr(c, err)
			return
		}
		cancelled[strconv.FormatInt(id, 10)+d] = true
	}
	crows.Close()
	if err := crows.Err(); err != nil {
		serverErr(c, err)
		return
	}

	// One entry per lesson that took place, in date order; a member's total counts only
	// the lessons since the day they joined.
	// Lessons whose student window is still open are the only ones that can be unmarked;
	// every other missing mark counts as an automatic "absent".
	var lessonDates []string
	openLessons := map[string]bool{} // slotID+date
	var openDates []string
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		for _, sl := range all {
			key := strconv.FormatInt(sl.ID, 10) + d.Format(dateLayout)
			if !sl.occursOn(s.Group.Week1Monday, d) || cancelled[key] {
				continue
			}
			if started, open := lessonState(now, d, sl.StartTime); started {
				lessonDates = append(lessonDates, d.Format(dateLayout))
				if open {
					openLessons[key] = true
					openDates = append(openDates, d.Format(dateLayout))
				}
			}
		}
	}
	since := func(dates []string, joined string) int {
		return len(dates) - sort.SearchStrings(dates, joined)
	}

	// Marks can only exist for lessons that occurred, but a deleted-then-recreated schedule may
	// leave marks on days that no longer match; they are ignored by re-checking per slot/date.
	rows, err := a.db.Query(c, `
		SELECT m.user_id, m.full_name, to_char(m.joined_at AT TIME ZONE $6, 'YYYY-MM-DD'),
			a.slot_id, to_char(a.lesson_date,'YYYY-MM-DD'), COALESCE(a.status,'')
		FROM members m
		LEFT JOIN attendance a ON a.user_id = m.user_id AND a.lesson_date BETWEEN $2::date AND $3::date
		WHERE m.group_id=$1 AND ($4 OR m.user_id=$5)
		ORDER BY m.full_name, m.user_id`, s.Group.ID, from, to, s.Role == "headman", s.UserID, s.Group.Timezone)
	if err != nil {
		serverErr(c, err)
		return
	}
	defer rows.Close()
	slotByID := map[int64]slot{}
	for _, sl := range all {
		slotByID[sl.ID] = sl
	}
	out := []statRow{}
	idx := map[int64]int{}
	joinedAt := map[int64]string{}
	for rows.Next() {
		var uid int64
		var name, joined, dateStr, status string
		var slotID *int64
		var dateNullable *string
		if err := rows.Scan(&uid, &name, &joined, &slotID, &dateNullable, &status); err != nil {
			serverErr(c, err)
			return
		}
		i, seen := idx[uid]
		if !seen {
			i = len(out)
			idx[uid] = i
			joinedAt[uid] = joined
			out = append(out, statRow{UserID: uid, FullName: name, Total: since(lessonDates, joined), Unmarked: since(openDates, joined)})
		}
		if slotID == nil || dateNullable == nil || *dateNullable < joinedAt[uid] {
			continue
		}
		dateStr = *dateNullable
		d, _ := parseDate(dateStr)
		sl, ok := slotByID[*slotID]
		if !ok || !sl.occursOn(s.Group.Week1Monday, d) || cancelled[strconv.FormatInt(*slotID, 10)+dateStr] {
			continue
		}
		if status == "present" {
			out[i].Present++
		}
		if openLessons[strconv.FormatInt(*slotID, 10)+dateStr] {
			out[i].Unmarked-- // marked while the window is still open
		}
	}
	if err := rows.Err(); err != nil {
		serverErr(c, err)
		return
	}
	for i := range out {
		out[i].Unmarked = max(0, out[i].Unmarked)
		out[i].Absent = max(0, out[i].Total-out[i].Present-out[i].Unmarked)
	}
	c.JSON(http.StatusOK, gin.H{"from": from.Format(dateLayout), "to": to.Format(dateLayout), "lessons": len(lessonDates), "rows": out})
}
