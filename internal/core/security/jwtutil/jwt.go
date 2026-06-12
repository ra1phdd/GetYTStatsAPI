package jwtutil

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

var rawEncoding = base64.RawURLEncoding

func SignHS256(claims any, secret string) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal jwt payload: %w", err)
	}

	encodedHeader := rawEncoding.EncodeToString(headerJSON)
	encodedPayload := rawEncoding.EncodeToString(payloadJSON)
	unsigned := encodedHeader + "." + encodedPayload
	return unsigned + "." + sign(unsigned, secret), nil
}

func ParseHS256(token string, secret string, claims any) error {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid jwt token format")
	}

	unsigned := parts[0] + "." + parts[1]
	expected := sign(unsigned, secret)
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expected)) != 1 {
		return fmt.Errorf("invalid jwt signature")
	}

	payload, err := rawEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("decode jwt payload: %w", err)
	}

	if err := json.Unmarshal(payload, claims); err != nil {
		return fmt.Errorf("unmarshal jwt payload: %w", err)
	}

	return nil
}

func sign(unsigned string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	return rawEncoding.EncodeToString(mac.Sum(nil))
}
