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
	sl, err := a.applyMark(ctx, s, s.UserID, slotID, date, status)
	if err != nil {
		var ue *userError
		if errors.As(err, &ue) {
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

func callbackRefusal(msg string) string {
	switch msg {
	case "marking is closed for this lesson":
		return "Отметка по этой паре уже закрыта"
	case "marking has not opened yet":
		return "Отметка ещё не открылась"
	case "lesson is cancelled":
		return "Пара отменена"
	case "lesson not found":
		return "Пары больше нет в расписании"
	}
	return "Не получилось отметиться"
}
