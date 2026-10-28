package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type config struct {
	Addr        string
	DatabaseURL string
	BotToken    string
	WebAppURL   string // public https URL of the Mini App
	BotUsername string // without @, used to build invite links
	AppShort    string // optional Mini App short name (for t.me/bot/app?startapp=)
	DevAuth     bool   // accept X-Dev-User header instead of Telegram initData (local only)
	StaticDir   string // built Mini App to serve alongside the API (single-service deploys)
	BotPolling  bool   // BOT_MODE=polling: long polling for local runs without public HTTPS
	// Admins see every user and group. Matched against Telegram-signed initData, so they
	// cannot be spoofed; IDs are stable, usernames are convenient.
	AdminIDs       map[int64]bool
	AdminUsernames map[string]bool
}

func (c config) isAdmin(u *tgUser) bool {
	return u != nil && (c.AdminIDs[u.ID] || (u.Username != "" && c.AdminUsernames[strings.ToLower(u.Username)]))
}

// parseList splits a comma/space separated env value.
func parseList(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
}

func loadConfig() (config, error) {
	c := config{
		Addr:        listenAddr(),
		StaticDir:   os.Getenv("STATIC_DIR"),
		BotPolling:  os.Getenv("BOT_MODE") == "polling",
		DatabaseURL: os.Getenv("DATABASE_URL"),
		BotToken:    os.Getenv("BOT_TOKEN"),
		WebAppURL:   strings.TrimRight(os.Getenv("WEBAPP_URL"), "/"),
		BotUsername: strings.TrimPrefix(os.Getenv("BOT_USERNAME"), "@"),
		AppShort:    os.Getenv("MINIAPP_SHORT_NAME"),
		DevAuth:     os.Getenv("DEV_AUTH") == "true",

		AdminIDs:       map[int64]bool{},
		AdminUsernames: map[string]bool{},
	}
	for _, s := range parseList(os.Getenv("ADMIN_IDS")) {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return c, fmt.Errorf("ADMIN_IDS: %q is not a Telegram user id", s)
		}
		c.AdminIDs[id] = true
	}
	for _, s := range parseList(os.Getenv("ADMIN_USERNAMES")) {
		c.AdminUsernames[strings.ToLower(strings.TrimPrefix(s, "@"))] = true
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if c.BotToken == "" && !c.DevAuth {
		return c, fmt.Errorf("BOT_TOKEN is required")
	}
	if c.BotToken != "" && !c.BotPolling && !strings.HasPrefix(c.WebAppURL, "https://") {
		return c, fmt.Errorf("WEBAPP_URL must be an https URL for the Telegram webhook (or set BOT_MODE=polling)")
	}
	return c, nil
}

// listenAddr honours PORT, which platforms such as Railway inject, then ADDR.
func listenAddr() string {
	if p := os.Getenv("PORT"); p != "" {
		return ":" + p
	}
	return envOr("ADDR", ":8080")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// inviteLink builds the deep link a headman shares with students.
// Main Mini App: t.me/<bot>?startapp=CODE; named Mini App: t.me/<bot>/<short>?startapp=CODE.
func (c config) inviteLink(code string) string {
	if c.BotUsername == "" {
		return ""
	}
	if c.AppShort != "" {
		return fmt.Sprintf("https://t.me/%s/%s?startapp=%s", c.BotUsername, c.AppShort, code)
	}
	return fmt.Sprintf("https://t.me/%s?startapp=%s", c.BotUsername, code)
}
