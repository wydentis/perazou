package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxScheduleFile = 10 << 20

var driveIDRe = regexp.MustCompile(`(?:/d/|[?&]id=)([A-Za-z0-9_-]{20,})`)

var errNotPublic = errors.New("файл недоступен по ссылке: откройте доступ «Все, у кого есть ссылка» или загрузите файл вручную")

// fetchGoogleFile downloads a Google Sheets / Drive file as .xlsx. Only the file ID is taken
// from the user's link; the request URLs are built here, so arbitrary hosts are never fetched.
func fetchGoogleFile(ctx context.Context, link string) ([]byte, error) {
	m := driveIDRe.FindStringSubmatch(link)
	if m == nil || !(strings.Contains(link, "docs.google.com") || strings.Contains(link, "drive.google.com")) {
		return nil, errors.New("нужна ссылка на Google Таблицу или файл на Google Диске")
	}
	id := m[1]
	candidates := []string{
		// Native Google Sheets.
		"https://docs.google.com/spreadsheets/d/" + id + "/export?format=xlsx",
		// Uploaded .xlsx files (links with rtpof=true open them in Sheets but they are Drive files).
		"https://drive.google.com/uc?export=download&id=" + id,
	}
	client := &http.Client{Timeout: 20 * time.Second}
	for _, u := range candidates {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxScheduleFile+1))
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		if len(body) > maxScheduleFile {
			return nil, errors.New("файл больше 10 МБ")
		}
		// An .xlsx is a zip archive; anything else is usually a Google sign-in page.
		if bytes.HasPrefix(body, []byte("PK")) {
			return body, nil
		}
	}
	return nil, errNotPublic
}

// parseImport accepts either a multipart upload ("file") or JSON {"url": "..."} and returns
// every group found in the timetable; the client picks one and previews it.
func (a *app) parseImport(c *gin.Context) {
	var data []byte
	if strings.HasPrefix(c.ContentType(), "multipart/") {
		fh, err := c.FormFile("file")
		if err != nil {
			fail(c, http.StatusBadRequest, "файл не передан")
			return
		}
		if fh.Size > maxScheduleFile {
			fail(c, http.StatusBadRequest, "файл больше 10 МБ")
			return
		}
		f, err := fh.Open()
		if err != nil {
			serverErr(c, err)
			return
		}
		defer f.Close()
		if data, err = io.ReadAll(io.LimitReader(f, maxScheduleFile)); err != nil {
			serverErr(c, err)
			return
		}
	} else {
		var req struct {
			URL string `json:"url"`
		}
		if !bind(c, &req) {
			return
		}
		var err error
		if data, err = fetchGoogleFile(c, req.URL); err != nil {
			fail(c, http.StatusBadRequest, err.Error())
			return
		}
	}
	groups, err := parseSchedule(bytes.NewReader(data))
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

// applyImport inserts the previewed slots. With replace=true the current schedule (and,
// through ON DELETE CASCADE, its attendance history) is removed first. A slot that already
// exists (same day, time, subject, parity and period) only gets its room and teacher
// refreshed, so re-importing an updated timetable is harmless.
func (a *app) applyImport(c *gin.Context) {
	s := sess(c)
	var req struct {
		Slots   []slotReq `json:"slots"`
		Replace bool      `json:"replace"`
	}
	if !bind(c, &req) {
		return
	}
	if len(req.Slots) == 0 || len(req.Slots) > 300 {
		fail(c, http.StatusBadRequest, "нужно от 1 до 300 пар")
		return
	}
	for i := range req.Slots {
		if !req.Slots[i].validate() {
			fail(c, http.StatusBadRequest, fmt.Sprintf("пара №%d заполнена неверно", i+1))
			return
		}
	}
	tx, err := a.db.Begin(c)
	if err != nil {
		serverErr(c, err)
		return
	}
	defer tx.Rollback(c)
	if req.Replace {
		if _, err := tx.Exec(c, `DELETE FROM schedule_slots WHERE group_id=$1`, s.Group.ID); err != nil {
			serverErr(c, err)
			return
		}
	}
	created := 0
	for _, r := range req.Slots {
		args := []any{s.Group.ID, r.Weekday, r.StartTime, r.EndTime, r.Subject, r.Room, r.Teacher, r.Parity, r.ValidFrom, r.ValidTo}
		tag, err := tx.Exec(c, `
			UPDATE schedule_slots SET room=$6, teacher=$7
			WHERE group_id=$1 AND weekday=$2 AND start_time=$3::time AND end_time=$4::time AND subject=$5 AND parity=$8
				AND valid_from IS NOT DISTINCT FROM NULLIF($9,'')::date AND valid_to IS NOT DISTINCT FROM NULLIF($10,'')::date`, args...)
		if err != nil {
			serverErr(c, err)
			return
		}
		if tag.RowsAffected() > 0 {
			continue
		}
		if _, err := tx.Exec(c, `
			INSERT INTO schedule_slots (group_id, weekday, start_time, end_time, subject, room, teacher, parity, valid_from, valid_to)
			VALUES ($1,$2,$3::time,$4::time,$5,$6,$7,$8,NULLIF($9,'')::date,NULLIF($10,'')::date)`, args...); err != nil {
			serverErr(c, err)
			return
		}
		created++
	}
	if err := tx.Commit(c); err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"created": created, "skipped": len(req.Slots) - created})
}
