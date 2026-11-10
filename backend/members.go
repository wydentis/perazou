package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// memberAttendance is one member's full history: every lesson that has started since they
// joined, with its mark. The headman can open anyone in the group; a student only themselves.
func (a *app) memberAttendance(c *gin.Context) {
	s := sess(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || (id != s.UserID && s.Role != "headman") {
		fail(c, http.StatusForbidden, "forbidden")
		return
	}
	var m memberInfo
	err = a.db.QueryRow(c, `
		SELECT m.user_id, m.full_name, m.role, COALESCE(u.username, ''), m.self_mark_blocked
		FROM members m JOIN users u ON u.id = m.user_id WHERE m.user_id=$1 AND m.group_id=$2`, id, s.Group.ID).
		Scan(&m.UserID, &m.FullName, &m.Role, &m.Username, &m.SelfMarkBlocked)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(c, http.StatusNotFound, "member not found")
		return
	} else if err != nil {
		serverErr(c, err)
		return
	}
	lessons, err := a.memberLessons(c, s, id)
	if err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"member": m, "lessons": lessons})
}

// setSelfMarkBlocked adds a student to, or removes them from, the list of members only the
// headman can mark. The headman cannot block themselves.
func (a *app) setSelfMarkBlocked(c *gin.Context) {
	s := sess(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Blocked bool `json:"blocked"`
	}
	if !bind(c, &req) {
		return
	}
	tag, err := a.db.Exec(c, `UPDATE members SET self_mark_blocked=$3 WHERE user_id=$1 AND group_id=$2 AND role <> 'headman'`,
		id, s.Group.ID, req.Blocked)
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
