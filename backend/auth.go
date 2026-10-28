package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type tgUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

var errBadInitData = errors.New("invalid init data")

// validateInitData checks Telegram Mini App initData per
// https://core.telegram.org/bots/webapps#validating-data-received-via-the-mini-app
// and returns the embedded user. maxAge bounds how stale auth_date may be.
func validateInitData(initData, botToken string, maxAge time.Duration, now time.Time) (*tgUser, error) {
	vals, err := url.ParseQuery(initData)
	if err != nil {
		return nil, errBadInitData
	}
	hash := vals.Get("hash")
	if hash == "" {
		return nil, errBadInitData
	}
	pairs := make([]string, 0, len(vals))
	for k, v := range vals {
		if k == "hash" {
			continue
		}
		pairs = append(pairs, k+"="+v[0])
	}
	sort.Strings(pairs)

	secret := hmacSHA256([]byte("WebAppData"), []byte(botToken))
	want := hex.EncodeToString(hmacSHA256(secret, []byte(strings.Join(pairs, "\n"))))
	if !hmac.Equal([]byte(want), []byte(strings.ToLower(hash))) {
		return nil, errBadInitData
	}

	authDate, err := strconv.ParseInt(vals.Get("auth_date"), 10, 64)
	if err != nil {
		return nil, errBadInitData
	}
	if now.Sub(time.Unix(authDate, 0)) > maxAge {
		return nil, errors.New("init data expired")
	}

	var u tgUser
	if err := json.Unmarshal([]byte(vals.Get("user")), &u); err != nil || u.ID == 0 {
		return nil, errBadInitData
	}
	return &u, nil
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}
