package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// runNotifier reminds members to mark themselves once a lesson has started. It polls
// rather than scheduling exact times so that a restart or redeploy simply catches up:
// any lesson that has started and whose marking window is still open gets reminders for
// members who have neither marked nor been reminded yet.
func (a *app) runNotifier(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		if err := a.notifyDue(ctx); err != nil && ctx.Err() == nil {
			log.Println("notifier:", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (a *app) notifyDue(ctx context.Context) error {
	rows, err := a.db.Query(ctx, `SELECT id, name, timezone FROM groups`)
	if err != nil {
		return err
	}
	var groups []groupRow
	for rows.Next() {
		var g groupRow
		if err := rows.Scan(&g.ID, &g.Name, &g.Timezone); err != nil {
			rows.Close()
			return err
		}
		groups = append(groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, g := range groups {
		if err := a.notifyGroup(ctx, g); err != nil {
			log.Printf("notifier: group %d: %v", g.ID, err)
		}
	}
	return nil
}

func (a *app) notifyGroup(ctx context.Context, g groupRow) error {
	now := g.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dateStr := today.Format(dateLayout)
	all, err := a.slots(ctx, g.ID)
	if err != nil {
		return err
	}
	for _, sl := range all {
		if started, open := lessonState(now, today, sl.StartTime); !sl.occursOn(today) || !started || !open {
			continue
		}
		// Claim the reminder rows first: ON CONFLICT makes concurrent replicas skip members
		// another instance already handled.
		claimed, err := a.db.Query(ctx, `
			INSERT INTO lesson_notifications (slot_id, lesson_date, user_id)
			SELECT $1, $2::date, m.user_id FROM members m
			WHERE m.group_id = $3 AND (m.joined_at AT TIME ZONE $4)::date <= $2::date
				AND NOT EXISTS (SELECT 1 FROM lesson_cancellations c WHERE c.slot_id = $1 AND c.lesson_date = $2::date)
				AND NOT EXISTS (SELECT 1 FROM attendance a WHERE a.slot_id = $1 AND a.lesson_date = $2::date AND a.user_id = m.user_id)
			ON CONFLICT DO NOTHING
			RETURNING user_id`, sl.ID, dateStr, g.ID, g.Timezone)
		if err != nil {
			return err
		}
		var users []int64
		for claimed.Next() {
			var uid int64
			if err := claimed.Scan(&uid); err != nil {
				claimed.Close()
				return err
			}
			users = append(users, uid)
		}
		claimed.Close()
		if err := claimed.Err(); err != nil {
			return err
		}
		for _, uid := range users {
			if err := a.bot.call(ctx, "sendMessage", a.reminder(uid, sl), nil); err != nil {
				if retryable(err) {
					// Release the claim so the next tick tries again while the window is open.
					a.db.Exec(ctx, `DELETE FROM lesson_notifications WHERE slot_id=$1 AND lesson_date=$2::date AND user_id=$3`, sl.ID, dateStr, uid)
				}
				log.Printf("notifier: user %d: %v", uid, err)
			}
			// Stay well under Telegram's ~30 messages/second limit.
			time.Sleep(50 * time.Millisecond)
		}
	}
	return nil
}

func (a *app) reminder(userID int64, sl slot) map[string]any {
	text := fmt.Sprintf("🔔 Началась пара «%s» в %s", sl.Subject, sl.StartTime)
	if sl.Room != "" {
		text += ", ауд. " + sl.Room
	}
	text += fmt.Sprintf(".\nОтметься до %s — потом будет автоматически «не был».", markClosesAt(sl.StartTime))
	return map[string]any{
		"chat_id": userID,
		"text":    text,
		"reply_markup": map[string]any{"inline_keyboard": [][]map[string]any{{
			{"text": "Отметиться", "web_app": map[string]string{"url": a.cfg.WebAppURL}},
		}}},
	}
}

// retryable reports whether a failed send may succeed later. Telegram refuses for good
// when the user blocked the bot or never allowed it to write ("Forbidden", "chat not found").
func retryable(err error) bool {
	msg := err.Error()
	return !strings.Contains(msg, "Forbidden") && !strings.Contains(msg, "chat not found")
}
