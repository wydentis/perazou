package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// bot is a minimal Telegram bot whose only job is to open the Mini App; all attendance
// functionality lives in the Mini App itself.
//
// In production it receives updates by webhook at webhookPath. Long polling remains for
// local development, where there is no public HTTPS URL for Telegram to call.
type bot struct {
	token  string
	webURL string
	secret string // X-Telegram-Bot-Api-Secret-Token expected on webhook calls
	http   *http.Client
}

const webhookPath = "/telegram/webhook"

// telegramAPI is a var so a test build can point it elsewhere (-ldflags -X main.telegramAPI=…).
var telegramAPI = "https://api.telegram.org"

func newBot(token, webURL string) *bot {
	// Derived from the token so every replica agrees without extra configuration, and it
	// changes whenever the token is rotated. Telegram allows [A-Za-z0-9_-]{1,256}.
	sum := sha256.Sum256([]byte("perazou-webhook:" + token))
	return &bot{token: token, webURL: webURL, secret: hex.EncodeToString(sum[:]), http: &http.Client{Timeout: 45 * time.Second}}
}

func (b *bot) call(ctx context.Context, method string, payload any, out any) error {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, telegramAPI+"/bot"+b.token+"/"+method, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		// *url.Error embeds the request URL, which contains the bot token.
		return fmt.Errorf("telegram %s: %s", method, strings.ReplaceAll(err.Error(), b.token, "<token>"))
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

// run registers the bot with Telegram and, in polling mode, consumes updates until ctx ends.
func (b *bot) run(ctx context.Context, polling bool) {
	// The menu button next to the input field opens the Mini App too.
	if err := b.call(ctx, "setChatMenuButton", map[string]any{
		"menu_button": map[string]any{"type": "web_app", "text": "Посещаемость", "web_app": map[string]string{"url": b.webURL}},
	}, nil); err != nil {
		log.Println("bot:", err)
	}
	if polling {
		b.poll(ctx)
		return
	}
	b.registerWebhook(ctx)
}

// registerWebhook points Telegram at this deployment. It retries with backoff because on a
// fresh deploy the domain or its certificate may not be ready yet. setWebhook is idempotent,
// so overlapping instances during a redeploy simply re-register the same URL.
func (b *bot) registerWebhook(ctx context.Context) {
	payload := map[string]any{
		"url":             b.webURL + webhookPath,
		"secret_token":    b.secret,
		"allowed_updates": []string{"message"},
	}
	for delay := 5 * time.Second; ; delay = min(delay*2, 5*time.Minute) {
		err := b.call(ctx, "setWebhook", payload, nil)
		if err == nil {
			log.Println("bot: webhook registered at", b.webURL+webhookPath)
			return
		}
		log.Printf("bot: %v (retrying in %s)", err, delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
	}
}

func (b *bot) poll(ctx context.Context) {
	// Long polling and webhooks are mutually exclusive.
	if err := b.call(ctx, "deleteWebhook", map[string]any{}, nil); err != nil {
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
			if reply := b.replyTo(u); reply != nil {
				if err := b.call(ctx, "sendMessage", reply, nil); err != nil {
					log.Println("bot:", err)
				}
			}
		}
	}
}

// webhook handles an update pushed by Telegram. The reply is returned in the response body
// as a Bot API method call, which saves a separate sendMessage request.
func (b *bot) webhook(c *gin.Context) {
	got := c.GetHeader("X-Telegram-Bot-Api-Secret-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(b.secret)) != 1 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var u update
	if err := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)).Decode(&u); err != nil {
		// Acknowledge anyway: a non-2xx makes Telegram redeliver the same update forever.
		c.Status(http.StatusOK)
		return
	}
	reply := b.replyTo(u)
	if reply == nil {
		c.Status(http.StatusOK)
		return
	}
	reply["method"] = "sendMessage"
	c.JSON(http.StatusOK, reply)
}

// replyTo builds the sendMessage payload for an update, or nil if it needs no answer.
func (b *bot) replyTo(u update) map[string]any {
	if u.Message == nil || u.Message.Chat.Type != "private" {
		return nil
	}
	target := b.webURL
	// "/start CODE" (plain start deep link) -> pass the invite code through the query string.
	if f := strings.Fields(u.Message.Text); len(f) == 2 && f[0] == "/start" {
		target += "?code=" + url.QueryEscape(f[1])
	}
	return map[string]any{
		"chat_id": u.Message.Chat.ID,
		"text":    "Отмечайте посещаемость в приложении 👇",
		"reply_markup": map[string]any{"inline_keyboard": [][]map[string]any{{
			{"text": "Открыть", "web_app": map[string]string{"url": target}},
		}}},
	}
}
