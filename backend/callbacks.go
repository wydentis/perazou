package main

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"
)

// markCallback handles the reminder's Был / Не был buttons: callback data "m|<slot>|<date>|<status>".
// The presser marks only themselves, under the same rules as in the Mini App.
func (a *app) markCallback(ctx context.Context, cq *callbackQuery) callbackReply {
	parts := strings.Split(cq.Data, "|")
	if len(parts) != 4 || parts[0] != "m" || (parts[3] != "present" && parts[3] != "absent") {
		return callbackReply{Answer: "Кнопка устарела"}
	}
	slotID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return callbackReply{Answer: "Кнопка устарела"}
	}
	date, status := parts[2], parts[3]
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	s, err := a.loadSession(ctx, cq.From.ID)
	if err != nil {
		log.Println("callback:", err)
		return callbackReply{Answer: "Не получилось, попробуйте ещё раз"}
	}
	if s == nil {
		return callbackReply{Answer: "Вы не состоите в группе"}
	}
	// The chat buttons belong to the reminder, so they work only inside the marking window,
	// for everyone: the headman's and correction-mode edits of past lessons stay in the app.
	if sl, d, err := a.loadLesson(ctx, s, slotID, date); err == nil {
		if _, open := lessonState(s.Group.now(), d, sl.StartTime); !open {
			return a.closedReply(ctx, sl, date, s.UserID)
		}
	}
	sl, err := a.applyMark(ctx, s, s.UserID, slotID, date, status)
	if err != nil {
		var ue *userError
		if errors.As(err, &ue) {
			if ue.msg == "marking is closed for this lesson" {
				return a.closedReply(ctx, sl, date, s.UserID)
			}
			return callbackReply{Answer: callbackRefusal(ue.msg)}
		}
		log.Println("callback:", err)
		return callbackReply{Answer: "Не получилось, попробуйте ещё раз"}
	}
	label := "✅ Отмечено: был"
	if status == "absent" {
		label = "❌ Отмечено: не был"
	}
	return callbackReply{
		Answer:   label,
		Text:     reminderText(sl) + "\n\n" + label + " (можно изменить до " + markClosesAt(sl.StartTime) + ")",
		Keyboard: a.reminderKeyboard(sl, date),
	}
}

// closedReply answers a press after the window: a popup, and the message loses its buttons.
func (a *app) closedReply(ctx context.Context, sl slot, date string, userID int64) callbackReply {
	var status string
	if err := a.db.QueryRow(ctx, `SELECT COALESCE((SELECT status FROM attendance WHERE slot_id=$1 AND lesson_date=$2::date AND user_id=$3), '')`,
		sl.ID, date, userID).Scan(&status); err != nil {
		log.Println("callback:", err)
	}
	return callbackReply{
		Answer:   "⏰ Время отметки закончилось в " + markClosesAt(sl.StartTime),
		Alert:    true,
		Text:     closedText(sl, status),
		Keyboard: a.appKeyboard(),
	}
}

func callbackRefusal(msg string) string {
	switch msg {
	case "marking is closed for this lesson":
		return "Отметка по этой паре уже закрыта"
	case "marking has not opened yet":
		return "Отметка ещё не открылась"
	case "lesson is cancelled":
		return "Пара отменена"
	case "self marking disabled":
		return "Вас отмечает староста"
	case "lesson not found":
		return "Пары больше нет в расписании"
	}
	return "Не получилось отметиться"
}
