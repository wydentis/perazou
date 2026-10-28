package main

import (
	"encoding/hex"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func signed(token string, authDate time.Time, userJSON string) string {
	v := url.Values{}
	v.Set("auth_date", strconv.FormatInt(authDate.Unix(), 10))
	v.Set("user", userJSON)
	v.Set("query_id", "AAH")
	secret := hmacSHA256([]byte("WebAppData"), []byte(token))
	check := "auth_date=" + v.Get("auth_date") + "\nquery_id=AAH\nuser=" + userJSON
	v.Set("hash", hex.EncodeToString(hmacSHA256(secret, []byte(check))))
	return v.Encode()
}

func TestValidateInitData(t *testing.T) {
	now := time.Now()
	user := `{"id":42,"first_name":"Ann"}`

	u, err := validateInitData(signed("tok", now, user), "tok", time.Hour, now)
	if err != nil || u.ID != 42 {
		t.Fatalf("valid data rejected: %v", err)
	}
	if _, err := validateInitData(signed("tok", now, user), "other", time.Hour, now); err == nil {
		t.Fatal("wrong token accepted")
	}
	if _, err := validateInitData(signed("tok", now.Add(-2*time.Hour), user), "tok", time.Hour, now); err == nil {
		t.Fatal("expired data accepted")
	}
	tampered := signed("tok", now, user) + "&extra=1"
	if _, err := validateInitData(tampered, "tok", time.Hour, now); err == nil {
		t.Fatal("tampered data accepted")
	}
}
