package serviceauth

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	HeaderServiceID = "X-Service-Id"
	HeaderTimestamp = "X-Timestamp"
	HeaderNonce     = "X-Nonce"
	HeaderSignature = "X-Signature"

	defaultMaxSkew = 5 * time.Minute
)

type NonceStore struct {
	mu     sync.Mutex
	values map[string]time.Time
	maxAge time.Duration
}

func NewNonceStore(maxAge time.Duration) *NonceStore {
	if maxAge <= 0 {
		maxAge = defaultMaxSkew
	}
	return &NonceStore{
		values: make(map[string]time.Time),
		maxAge: maxAge,
	}
}

func (s *NonceStore) Use(nonce string, now time.Time) bool {
	if s == nil || nonce == "" {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := now.Add(-s.maxAge)
	for key, value := range s.values {
		if value.Before(cutoff) {
			delete(s.values, key)
		}
	}

	if _, exists := s.values[nonce]; exists {
		return false
	}

	s.values[nonce] = now
	return true
}

func (s *NonceStore) Forget(nonce string) {
	if s == nil || nonce == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, nonce)
}

func SignRequest(req *http.Request, serviceID string, secret string) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	body, err := readAndRestoreBody(req)
	if err != nil {
		return err
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)
	nonce, err := randomHex(16)
	if err != nil {
		return err
	}

	signature := BuildSignature(req.Method, req.URL.Path, req.URL.RawQuery, body, timestamp, nonce, serviceID, secret)
	req.Header.Set(HeaderServiceID, serviceID)
	req.Header.Set(HeaderTimestamp, timestamp)
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSignature, signature)
	return nil
}

func VerifyRequest(req *http.Request, expectedServiceID string, secret string, nonces *NonceStore) error {
	if req == nil {
		return fmt.Errorf("request is nil")
	}

	serviceID := strings.TrimSpace(req.Header.Get(HeaderServiceID))
	if serviceID == "" || serviceID != strings.TrimSpace(expectedServiceID) {
		return fmt.Errorf("unexpected service id")
	}

	timestampRaw := strings.TrimSpace(req.Header.Get(HeaderTimestamp))
	if timestampRaw == "" {
		return fmt.Errorf("missing timestamp")
	}
	timestamp, err := time.Parse(time.RFC3339, timestampRaw)
	if err != nil {
		return fmt.Errorf("parse timestamp: %w", err)
	}
	now := time.Now().UTC()
	if timestamp.Before(now.Add(-defaultMaxSkew)) || timestamp.After(now.Add(defaultMaxSkew)) {
		return fmt.Errorf("request timestamp is outside allowed skew")
	}

	nonce := strings.TrimSpace(req.Header.Get(HeaderNonce))
	if nonce == "" {
		return fmt.Errorf("missing nonce")
	}
	if !nonces.Use(serviceID+":"+nonce, now) {
		return fmt.Errorf("nonce already used")
	}

	body, err := readAndRestoreBody(req)
	if err != nil {
		return err
	}

	expected := BuildSignature(req.Method, req.URL.Path, req.URL.RawQuery, body, timestampRaw, nonce, serviceID, secret)
	provided := strings.TrimSpace(req.Header.Get(HeaderSignature))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

func BuildSignature(method string, path string, rawQuery string, body []byte, timestamp string, nonce string, serviceID string, secret string) string {
	bodyHash := sha256.Sum256(body)
	payload := strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(method)),
		path,
		rawQuery,
		hex.EncodeToString(bodyHash[:]),
		timestamp,
		nonce,
		serviceID,
	}, "\n")

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func readAndRestoreBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random nonce: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
