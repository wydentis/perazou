package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (a *app) adminOnly(c *gin.Context) {
	if !a.cfg.isAdmin(c.MustGet(keyUser).(*tgUser)) {
		fail(c, http.StatusForbidden, "admin only")
	}
}

type adminUser struct {
	ID        int64      `json:"id"`
	Username  string     `json:"username"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
	FullName  string     `json:"full_name"` // name inside the group, "" when not in one
	Role      string     `json:"role"`
	GroupID   *int64     `json:"group_id"`
	GroupName string     `json:"group_name"`
	CreatedAt time.Time  `json:"created_at"`
	JoinedAt  *time.Time `json:"joined_at"`
}

type adminGroup struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Headman   string    `json:"headman"`
	Members   int       `json:"members"`
	Slots     int       `json:"slots"`
	Marks     int       `json:"marks"` // explicit attendance marks over the last 30 days
	CreatedAt time.Time `json:"created_at"`
}

// adminOverview returns every user and group. The data set is small (one row per person),
// so it is sent whole and filtered on the client.
func (a *app) adminOverview(c *gin.Context) {
	users := []adminUser{}
	rows, err := a.db.Query(c, `
		SELECT u.id, u.username, u.first_name, u.last_name, COALESCE(m.full_name, ''), COALESCE(m.role, ''),
			g.id, COALESCE(g.name, ''), u.created_at, m.joined_at
		FROM users u
		LEFT JOIN members m ON m.user_id = u.id
		LEFT JOIN groups g ON g.id = m.group_id
		ORDER BY u.created_at DESC`)
	if err != nil {
		serverErr(c, err)
		return
	}
	for rows.Next() {
		var x adminUser
		if err := rows.Scan(&x.ID, &x.Username, &x.FirstName, &x.LastName, &x.FullName, &x.Role,
			&x.GroupID, &x.GroupName, &x.CreatedAt, &x.JoinedAt); err != nil {
			rows.Close()
			serverErr(c, err)
			return
		}
		users = append(users, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		serverErr(c, err)
		return
	}

	groups := []adminGroup{}
	grows, err := a.db.Query(c, `
		SELECT g.id, g.name, COALESCE(h.full_name, ''),
			(SELECT count(*) FROM members m WHERE m.group_id = g.id),
			(SELECT count(*) FROM schedule_slots s WHERE s.group_id = g.id),
			(SELECT count(*) FROM attendance a JOIN schedule_slots s ON s.id = a.slot_id
				WHERE s.group_id = g.id AND a.lesson_date >= current_date - 30),
			g.created_at
		FROM groups g
		LEFT JOIN members h ON h.group_id = g.id AND h.role = 'headman'
		ORDER BY g.created_at DESC`)
	if err != nil {
		serverErr(c, err)
		return
	}
	defer grows.Close()
	for grows.Next() {
		var g adminGroup
		if err := grows.Scan(&g.ID, &g.Name, &g.Headman, &g.Members, &g.Slots, &g.Marks, &g.CreatedAt); err != nil {
			serverErr(c, err)
			return
		}
		groups = append(groups, g)
	}
	if err := grows.Err(); err != nil {
		serverErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "groups": groups})
}
