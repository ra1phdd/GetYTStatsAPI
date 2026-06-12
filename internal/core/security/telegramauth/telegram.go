package telegramauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Result struct {
	UserID    int64  `json:"user_id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	PhotoURL  string `json:"photo_url,omitempty"`
	AuthDate  int64  `json:"auth_date"`
}

type LoginWidgetPayload struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date"`
	Hash      string `json:"hash"`
}

func ValidateLoginWidget(payload LoginWidgetPayload, botToken string, maxAge time.Duration) (Result, error) {
	fields := map[string]string{
		"auth_date":  strconv.FormatInt(payload.AuthDate, 10),
		"first_name": payload.FirstName,
		"id":         strconv.FormatInt(payload.ID, 10),
		"last_name":  payload.LastName,
		"photo_url":  payload.PhotoURL,
		"username":   payload.Username,
	}

	secret := sha256.Sum256([]byte(botToken))
	if err := validate(fields, payload.Hash, secret[:], maxAge); err != nil {
		return Result{}, err
	}

	return Result{
		UserID:    payload.ID,
		Username:  payload.Username,
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
		PhotoURL:  payload.PhotoURL,
		AuthDate:  payload.AuthDate,
	}, nil
}

func ValidateWebAppInitData(initData string, botToken string, maxAge time.Duration) (Result, error) {
	values, err := url.ParseQuery(strings.TrimSpace(initData))
	if err != nil {
		return Result{}, fmt.Errorf("parse init data: %w", err)
	}

	fields := make(map[string]string, len(values))
	for key, items := range values {
		if len(items) == 0 {
			continue
		}
		fields[key] = items[0]
	}

	hash := fields["hash"]
	if hash == "" {
		return Result{}, fmt.Errorf("missing init data hash")
	}

	mac := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = mac.Write([]byte(botToken))
	secret := mac.Sum(nil)
	if err := validate(fields, hash, secret, maxAge); err != nil {
		return Result{}, err
	}

	userID, _ := strconv.ParseInt(extractJSONNumber(fields["user"], "id"), 10, 64)
	return Result{
		UserID:    userID,
		Username:  extractJSONString(fields["user"], "username"),
		FirstName: extractJSONString(fields["user"], "first_name"),
		LastName:  extractJSONString(fields["user"], "last_name"),
		PhotoURL:  extractJSONString(fields["user"], "photo_url"),
		AuthDate:  parseInt64(fields["auth_date"]),
	}, nil
}

func validate(fields map[string]string, expectedHash string, secret []byte, maxAge time.Duration) error {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}

	authDate := parseInt64(fields["auth_date"])
	if authDate == 0 {
		return fmt.Errorf("missing auth_date")
	}
	if time.Unix(authDate, 0).Before(time.Now().Add(-maxAge)) {
		return fmt.Errorf("telegram auth payload expired")
	}

	dataCheckString := buildDataCheckString(fields)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(dataCheckString))
	actual := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(actual), []byte(strings.TrimSpace(expectedHash))) {
		return fmt.Errorf("invalid telegram auth hash")
	}

	return nil
}

func buildDataCheckString(fields map[string]string) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if key == "hash" || strings.TrimSpace(fields[key]) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+fields[key])
	}
	return strings.Join(parts, "\n")
}

func parseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed
}

func extractJSONString(source string, key string) string {
	prefix := fmt.Sprintf("\"%s\":\"", key)
	idx := strings.Index(source, prefix)
	if idx < 0 {
		return ""
	}
	value := source[idx+len(prefix):]
	end := strings.Index(value, "\"")
	if end < 0 {
		return ""
	}
	return value[:end]
}

func extractJSONNumber(source string, key string) string {
	prefix := fmt.Sprintf("\"%s\":", key)
	idx := strings.Index(source, prefix)
	if idx < 0 {
		return ""
	}
	value := source[idx+len(prefix):]
	end := strings.IndexAny(value, ",}")
	if end < 0 {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(value[:end])
}
