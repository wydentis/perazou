package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// bot is a minimal long-polling Telegram bot: its only job is to open the Mini App.
// All attendance functionality lives in the Mini App itself.
type bot struct {
	token  string
	webURL string
	http   *http.Client
}

func newBot(token, webURL string) *bot {
	return &bot{token: token, webURL: webURL, http: &http.Client{Timeout: 45 * time.Second}}
}

func (b *bot) call(ctx context.Context, method string, payload any, out any) error {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+b.token+"/"+method, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var r struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return err
	}
	if !r.OK {
		return fmt.Errorf("telegram %s: %s", method, r.Description)
	}
	if out != nil {
		return json.Unmarshal(r.Result, out)
	}
	return nil
}

type update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		Chat struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
	} `json:"message"`
}

func (b *bot) run(ctx context.Context) {
	if b.webURL == "" {
		log.Println("bot: WEBAPP_URL is empty, bot disabled")
		return
	}
	// Long polling and webhooks are mutually exclusive.
	if err := b.call(ctx, "deleteWebhook", map[string]any{}, nil); err != nil {
		log.Println("bot:", err)
	}
	// The menu button next to the input field opens the Mini App too.
	if err := b.call(ctx, "setChatMenuButton", map[string]any{
		"menu_button": map[string]any{"type": "web_app", "text": "Посещаемость", "web_app": map[string]string{"url": b.webURL}},
	}, nil); err != nil {
		log.Println("bot:", err)
	}

	var offset int64
	for ctx.Err() == nil {
		var updates []update
		err := b.call(ctx, "getUpdates", map[string]any{"offset": offset, "timeout": 30, "allowed_updates": []string{"message"}}, &updates)
		if err != nil {
			if ctx.Err() == nil {
				log.Println("bot:", err)
				time.Sleep(3 * time.Second)
			}
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message != nil && u.Message.Chat.Type == "private" {
				b.handleMessage(ctx, u)
			}
		}
	}
}

func (b *bot) handleMessage(ctx context.Context, u update) {
	target := b.webURL
	// "/start CODE" (plain start deep link) -> pass the invite code through the query string.
	if f := strings.Fields(u.Message.Text); len(f) == 2 && f[0] == "/start" {
		target += "?code=" + url.QueryEscape(f[1])
	}
	err := b.call(ctx, "sendMessage", map[string]any{
		"chat_id": u.Message.Chat.ID,
		"text":    "Отмечайте посещаемость в приложении 👇",
		"reply_markup": map[string]any{"inline_keyboard": [][]map[string]any{{
			{"text": "Открыть", "web_app": map[string]string{"url": target}},
		}}},
	}, nil)
	if err != nil {
		log.Println("bot:", err)
	}
}
