package campaign_telegram

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"getytstatsapi/internal/core/domain"
	"getytstatsapi/internal/core/security/serviceauth"
)

func TestWebhookHandlerIgnoresDuplicateEvent(t *testing.T) {
	t.Parallel()

	handler := NewWebhookHandler(nil, nil, "api", "secret")
	event := domain.BotWebhookEvent{
		EventID:   "evt-1",
		CreatedAt: time.Now().UTC(),
		Type:      domain.BotWebhookEventNotificationDigest,
		Digest:    &domain.NotificationDigest{UserID: 1},
	}
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	first := newSignedWebhookRequest(t, body)
	firstRecorder := httptest.NewRecorder()
	handler.ServeHTTP(firstRecorder, first)
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first webhook status = %d, want 200", firstRecorder.Code)
	}

	second := newSignedWebhookRequest(t, body)
	secondRecorder := httptest.NewRecorder()
	handler.ServeHTTP(secondRecorder, second)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("second webhook status = %d, want 200", secondRecorder.Code)
	}
	if !bytes.Contains(secondRecorder.Body.Bytes(), []byte("duplicate")) {
		t.Fatalf("second webhook body = %q, want duplicate marker", secondRecorder.Body.String())
	}
}

func newSignedWebhookRequest(t *testing.T, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://bot.local/v1/internal/webhooks/campaign-events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if err := serviceauth.SignRequest(req, "api", "secret"); err != nil {
		t.Fatalf("SignRequest() error = %v", err)
	}
	return req
}
