package telegramauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestValidateLoginWidget(t *testing.T) {
	t.Parallel()

	payload := LoginWidgetPayload{
		ID:        42,
		FirstName: "Ivan",
		LastName:  "Ivanov",
		Username:  "ivan",
		PhotoURL:  "https://example.com/avatar.jpg",
		AuthDate:  time.Now().Unix(),
	}
	fields := map[string]string{
		"auth_date":  strconv.FormatInt(payload.AuthDate, 10),
		"first_name": payload.FirstName,
		"id":         strconv.FormatInt(payload.ID, 10),
		"last_name":  payload.LastName,
		"photo_url":  payload.PhotoURL,
		"username":   payload.Username,
	}
	secret := sha256.Sum256([]byte("bot-token"))
	payload.Hash = signLoginWidget(fields, secret[:])

	result, err := ValidateLoginWidget(payload, "bot-token", time.Hour)
	if err != nil {
		t.Fatalf("ValidateLoginWidget() error = %v", err)
	}
	if result.UserID != payload.ID {
		t.Fatalf("result user id = %d, want %d", result.UserID, payload.ID)
	}
}

func signLoginWidget(fields map[string]string, secret []byte) string {
	data := buildDataCheckString(fields)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.TrimSpace(data)))
	return hex.EncodeToString(mac.Sum(nil))
}
