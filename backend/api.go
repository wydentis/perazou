package main

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type app struct {
	db  *pgxpool.Pool
	cfg config
}

type groupInfo struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Timezone    string `json:"timezone"`
	Week1Monday string `json:"week1_monday"`
	// Parity of the current week ("odd"/"even"), handy for the settings form.
	CurrentWeek string `json:"current_week"`
}

type memberInfo struct {
	UserID   int64  `json:"user_id"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	Username string `json:"username,omitempty"`
}

const (
	keyUser   = "user"
	keyMember = "member"
	keyGroup  = "group"
)

type session struct {
	UserID   int64
	FullName string
	Role     string
	Group    groupRow
}

type groupRow struct {
	ID          int64
	Name        string
	Code        string
	Timezone    string
	Week1Monday time.Time
}

func (g groupRow) loc() *time.Location {
	l, err := time.LoadLocation(g.Timezone)
	if err != nil {
		return time.UTC
	}
	return l
}

// now returns the wall-clock time in the group's timezone, re-labelled as UTC so that
// it can be compared directly with calendar dates parsed as UTC midnight.
func (g groupRow) now() time.Time {
	n := time.Now().In(g.loc())
	return time.Date(n.Year(), n.Month(), n.Day(), n.Hour(), n.Minute(), 0, 0, time.UTC)
}

func (g groupRow) info() groupInfo {
	cur := "even"
	if isOddWeek(g.Week1Monday, g.now()) {
		cur = "odd"
	}
	return groupInfo{ID: g.ID, Name: g.Name, Timezone: g.Timezone, Week1Monday: g.Week1Monday.Format(dateLayout), CurrentWeek: cur}
}

func newRouter(a *app) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	r.GET("/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	api := r.Group("/api", a.authMiddleware)
	api.GET("/me", a.getMe)
	api.GET("/groups/preview", a.previewGroup)
	api.POST("/groups", a.createGroup)
	api.POST("/groups/join", a.joinGroup)

	m := api.Group("", a.memberMiddleware)
	m.GET("/group", a.getGroup)
	m.PATCH("/group", headmanOnly, a.updateGroup)
	m.POST("/group/invite/regenerate", headmanOnly, a.regenerateInvite)
	m.POST("/group/leave", a.leaveGroup)
	m.PATCH("/members/:id", a.renameMember)
	m.DELETE("/members/:id", headmanOnly, a.removeMember)

	m.GET("/schedule", a.listSlots)
	m.POST("/schedule", headmanOnly, a.createSlot)
	m.PUT("/schedule/:id", headmanOnly, a.updateSlot)
	m.DELETE("/schedule/:id", headmanOnly, a.deleteSlot)
	m.POST("/schedule/import/parse", headmanOnly, a.parseImport)
	m.POST("/schedule/import/apply", headmanOnly, a.applyImport)

	m.GET("/lessons", a.listLessons)
	m.GET("/lessons/attendance", headmanOnly, a.lessonAttendance)
	m.PUT("/lessons/cancel", headmanOnly, a.cancelLesson)
	m.PUT("/attendance", a.markAttendance)
	m.GET("/stats", a.stats)
	return r
}

func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

func (a *app) authMiddleware(c *gin.Context) {
	var u *tgUser
	if a.cfg.DevAuth {
		// "id:Name", percent-encoded by the client so names may be Cyrillic.
		if h, err := url.PathUnescape(c.GetHeader("X-Dev-User")); err == nil && h != "" {
			id, name, _ := strings.Cut(h, ":")
			uid, err := strconv.ParseInt(id, 10, 64)
			if err == nil {
				u = &tgUser{ID: uid, FirstName: name}
			}
		}
	}
	if u == nil {
		initData, ok := strings.CutPrefix(c.GetHeader("Authorization"), "tma ")
		if !ok {
			fail(c, http.StatusUnauthorized, "missing credentials")
			return
		}
		var err error
		u, err = validateInitData(initData, a.cfg.BotToken, 24*time.Hour, time.Now())
		if err != nil {
			fail(c, http.StatusUnauthorized, err.Error())
			return
		}
	}
	_, err := a.db.Exec(c, `
		INSERT INTO users (id, first_name, last_name, username) VALUES ($1,$2,$3,$4)
		ON CONFLICT (id) DO UPDATE SET first_name=$2, last_name=$3, username=$4`,
		u.ID, u.FirstName, u.LastName, u.Username)
	if err != nil {
		fail(c, http.StatusInternalServerError, "db error")
		return
	}
	c.Set(keyUser, u)
}

func (a *app) loadSession(ctx context.Context, userID int64) (*session, error) {
	s := &session{UserID: userID}
	err := a.db.QueryRow(ctx, `
		SELECT m.full_name, m.role, g.id, g.name, g.invite_code, g.timezone, g.week1_monday
		FROM members m JOIN groups g ON g.id = m.group_id WHERE m.user_id = $1`, userID).
		Scan(&s.FullName, &s.Role, &s.Group.ID, &s.Group.Name, &s.Group.Code, &s.Group.Timezone, &s.Group.Week1Monday)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

func (a *app) memberMiddleware(c *gin.Context) {
	u := c.MustGet(keyUser).(*tgUser)
	s, err := a.loadSession(c, u.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "db error")
		return
	}
	if s == nil {
		fail(c, http.StatusForbidden, "not in a group")
		return
	}
	c.Set(keyMember, s)
}

func headmanOnly(c *gin.Context) {
	if sess(c).Role != "headman" {
		fail(c, http.StatusForbidden, "headman only")
	}
}

func sess(c *gin.Context) *session { return c.MustGet(keyMember).(*session) }

func bind(c *gin.Context, v any) bool {
	if err := c.ShouldBindJSON(v); err != nil {
		fail(c, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func serverErr(c *gin.Context, err error) {
	c.Error(err)
	fail(c, http.StatusInternalServerError, "internal error")
}

const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newInviteCode() (string, error) {
	b := make([]byte, 8)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = codeAlphabet[n.Int64()]
	}
	return string(b), nil
}

func cleanName(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}

// ---- me / groups ----

func (a *app) getMe(c *gin.Context) {
	u := c.MustGet(keyUser).(*tgUser)
	s, err := a.loadSession(c, u.ID)
	if err != nil {
		serverErr(c, err)
		return
	}
	out := gin.H{"user": u, "member": nil, "group": nil}
	if s != nil {
		out["member"] = memberInfo{UserID: s.UserID, FullName: s.FullName, Role: s.Role}
		out["group"] = s.Group.info()
	}
	c.JSON(http.StatusOK, out)
}

func (a *app) previewGroup(c *gin.Context) {
	var name string
	err := a.db.QueryRow(c, `SELECT name FROM groups WHERE invite_code = $1`, strings.ToUpper(strings.TrimSpace(c.Query("code")))).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(c, http.StatusNotFound, "invite not found")
		return
	} else if err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": name})
}

func (a *app) createGroup(c *gin.Context) {
	u := c.MustGet(keyUser).(*tgUser)
	var req struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		Timezone    string `json:"timezone"`
		CurrentWeek string `json:"current_week"` // odd | even
	}
	if !bind(c, &req) {
		return
	}
	req.Name, req.FullName = cleanName(req.Name, 64), cleanName(req.FullName, 100)
	if req.Name == "" || req.FullName == "" {
		fail(c, http.StatusBadRequest, "name and full_name are required")
		return
	}
	if req.Timezone == "" {
		req.Timezone = "Europe/Moscow"
	}
	loc, err := time.LoadLocation(req.Timezone)
	if err != nil || (req.CurrentWeek != "odd" && req.CurrentWeek != "even") {
		fail(c, http.StatusBadRequest, "invalid timezone or current_week")
		return
	}
	week1 := weekAnchor(loc, req.CurrentWeek == "odd")

	tx, err := a.db.Begin(c)
	if err != nil {
		serverErr(c, err)
		return
	}
	defer tx.Rollback(c)

	var gid int64
	for attempt := 0; ; attempt++ {
		code, err := newInviteCode()
		if err != nil {
			serverErr(c, err)
			return
		}
		// DO NOTHING on a code collision (no row returned) instead of failing the transaction.
		err = tx.QueryRow(c, `INSERT INTO groups (name, invite_code, timezone, week1_monday) VALUES ($1,$2,$3,$4)
			ON CONFLICT (invite_code) DO NOTHING RETURNING id`, req.Name, code, req.Timezone, week1).Scan(&gid)
		if err == nil {
			break
		}
		if !errors.Is(err, pgx.ErrNoRows) || attempt > 3 {
			serverErr(c, err)
			return
		}
	}
	if _, err := tx.Exec(c, `INSERT INTO members (user_id, group_id, full_name, role) VALUES ($1,$2,$3,'headman')`, u.ID, gid, req.FullName); err != nil {
		if isUnique(err) {
			fail(c, http.StatusConflict, "already in a group")
			return
		}
		serverErr(c, err)
		return
	}
	if err := tx.Commit(c); err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"group_id": gid})
}

// weekAnchor returns the Monday that should be stored as "week 1" so that the current
// week has the requested parity.
func weekAnchor(loc *time.Location, currentIsOdd bool) time.Time {
	n := time.Now().In(loc)
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	mon := mondayOf(today)
	if !currentIsOdd {
		mon = mon.AddDate(0, 0, -7)
	}
	return mon
}

func isUnique(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func (a *app) joinGroup(c *gin.Context) {
	u := c.MustGet(keyUser).(*tgUser)
	var req struct {
		Code     string `json:"code"`
		FullName string `json:"full_name"`
	}
	if !bind(c, &req) {
		return
	}
	req.FullName = cleanName(req.FullName, 100)
	if req.FullName == "" {
		fail(c, http.StatusBadRequest, "full_name is required")
		return
	}
	var gid int64
	err := a.db.QueryRow(c, `SELECT id FROM groups WHERE invite_code = $1`, strings.ToUpper(strings.TrimSpace(req.Code))).Scan(&gid)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(c, http.StatusNotFound, "invite not found")
		return
	} else if err != nil {
		serverErr(c, err)
		return
	}
	if _, err := a.db.Exec(c, `INSERT INTO members (user_id, group_id, full_name, role) VALUES ($1,$2,$3,'student')`, u.ID, gid, req.FullName); err != nil {
		if isUnique(err) {
			fail(c, http.StatusConflict, "already in a group")
			return
		}
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"group_id": gid})
}

func (a *app) getGroup(c *gin.Context) {
	s := sess(c)
	rows, err := a.db.Query(c, `
		SELECT m.user_id, m.full_name, m.role, u.username
		FROM members m JOIN users u ON u.id = m.user_id
		WHERE m.group_id = $1 ORDER BY (m.role = 'headman') DESC, m.full_name`, s.Group.ID)
	if err != nil {
		serverErr(c, err)
		return
	}
	defer rows.Close()
	members := []memberInfo{}
	for rows.Next() {
		var m memberInfo
		if err := rows.Scan(&m.UserID, &m.FullName, &m.Role, &m.Username); err != nil {
			serverErr(c, err)
			return
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		serverErr(c, err)
		return
	}
	out := gin.H{"group": s.Group.info(), "members": members}
	if s.Role == "headman" {
		out["invite_code"] = s.Group.Code
		out["invite_link"] = a.cfg.inviteLink(s.Group.Code)
	}
	c.JSON(http.StatusOK, out)
}

func (a *app) updateGroup(c *gin.Context) {
	s := sess(c)
	var req struct {
		Name        string `json:"name"`
		Timezone    string `json:"timezone"`
		CurrentWeek string `json:"current_week"`
	}
	if !bind(c, &req) {
		return
	}
	req.Name = cleanName(req.Name, 64)
	loc, err := time.LoadLocation(req.Timezone)
	if req.Name == "" || err != nil || (req.CurrentWeek != "odd" && req.CurrentWeek != "even") {
		fail(c, http.StatusBadRequest, "invalid name, timezone or current_week")
		return
	}
	// Re-anchor only if the requested parity differs from the current one (evaluated in the
	// new timezone); otherwise a stale form submitted across a week boundary would flip it.
	anchor := s.Group.Week1Monday
	moved := s.Group
	moved.Timezone = req.Timezone
	if isOddWeek(anchor, moved.now()) != (req.CurrentWeek == "odd") {
		anchor = weekAnchor(loc, req.CurrentWeek == "odd")
	}
	if _, err := a.db.Exec(c, `UPDATE groups SET name=$2, timezone=$3, week1_monday=$4 WHERE id=$1`,
		s.Group.ID, req.Name, req.Timezone, anchor); err != nil {
		serverErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *app) regenerateInvite(c *gin.Context) {
	s := sess(c)
	for i := 0; i < 5; i++ {
		code, err := newInviteCode()
		if err != nil {
			serverErr(c, err)
			return
		}
		if _, err = a.db.Exec(c, `UPDATE groups SET invite_code=$2 WHERE id=$1`, s.Group.ID, code); err == nil {
			c.JSON(http.StatusOK, gin.H{"invite_code": code, "invite_link": a.cfg.inviteLink(code)})
			return
		} else if !isUnique(err) {
			serverErr(c, err)
			return
		}
	}
	fail(c, http.StatusInternalServerError, "could not generate code")
}

func (a *app) leaveGroup(c *gin.Context) {
	s := sess(c)
	if s.Role == "headman" {
		fail(c, http.StatusBadRequest, "headman cannot leave the group")
		return
	}
	if _, err := a.db.Exec(c, `DELETE FROM members WHERE user_id=$1`, s.UserID); err != nil {
		serverErr(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *app) renameMember(c *gin.Context) {
	s := sess(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || (id != s.UserID && s.Role != "headman") {
		fail(c, http.StatusForbidden, "forbidden")
		return
	}
	var req struct {
		FullName string `json:"full_name"`
	}
	if !bind(c, &req) {
		return
	}
	name := cleanName(req.FullName, 100)
	if name == "" {
		fail(c, http.StatusBadRequest, "full_name is required")
		return
	}
	tag, err := a.db.Exec(c, `UPDATE members SET full_name=$3 WHERE user_id=$1 AND group_id=$2`, id, s.Group.ID, name)
	if err != nil {
		serverErr(c, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(c, http.StatusNotFound, "member not found")
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *app) removeMember(c *gin.Context) {
	s := sess(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id == s.UserID {
		fail(c, http.StatusBadRequest, "cannot remove this member")
		return
	}
	tag, err := a.db.Exec(c, `DELETE FROM members WHERE user_id=$1 AND group_id=$2`, id, s.Group.ID)
	if err != nil {
		serverErr(c, err)
		return
	}
	if tag.RowsAffected() == 0 {
		fail(c, http.StatusNotFound, "member not found")
		return
	}
	c.Status(http.StatusNoContent)
}
