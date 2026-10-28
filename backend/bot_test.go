package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWebhook(t *testing.T) {
	gin.SetMode(gin.TestMode)
	b := newBot("123:abc", "https://perazou.example")
	r := gin.New()
	r.POST(webhookPath, b.webhook)
	send := func(secret, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, webhookPath, strings.NewReader(body))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secret)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	start := `{"update_id":1,"message":{"text":"/start ABCD2345","chat":{"id":42,"type":"private"}}}`

	if w := send("wrong", start); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret: %d", w.Code)
	}

	w := send(b.secret, start)
	var reply struct {
		Method      string `json:"method"`
		ChatID      int64  `json:"chat_id"`
		ReplyMarkup struct {
			InlineKeyboard [][]struct {
				WebApp struct{ URL string } `json:"web_app"`
			} `json:"inline_keyboard"`
		} `json:"reply_markup"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil || w.Code != http.StatusOK {
		t.Fatalf("reply: %d %s", w.Code, w.Body)
	}
	if reply.Method != "sendMessage" || reply.ChatID != 42 ||
		reply.ReplyMarkup.InlineKeyboard[0][0].WebApp.URL != "https://perazou.example?code=ABCD2345" {
		t.Errorf("unexpected reply: %s", w.Body)
	}

	// Group messages and garbage are acknowledged without a reply, so Telegram does not retry them.
	group := `{"update_id":2,"message":{"text":"hi","chat":{"id":-1,"type":"group"}}}`
	for _, body := range []string{group, "not json"} {
		if w := send(b.secret, body); w.Code != http.StatusOK || w.Body.Len() != 0 {
			t.Errorf("%q: %d %q", body, w.Code, w.Body)
		}
	}
}

func TestProfileTextsFitTelegramLimits(t *testing.T) {
	if n := len([]rune(botShortDescription)); n > 120 {
		t.Errorf("short description is %d characters, limit 120", n)
	}
	if n := len([]rune(botDescription)); n > 512 {
		t.Errorf("description is %d characters, limit 512", n)
	}
}
