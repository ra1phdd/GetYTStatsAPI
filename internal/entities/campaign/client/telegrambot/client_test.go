package telegrambot_client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"getytstatsapi/internal/core/domain"
	"getytstatsapi/internal/core/security/serviceauth"
)

func TestSendDigestRetriesUntilSuccess(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if err := serviceauth.VerifyRequest(r, "api", "secret", serviceauth.NewNonceStore(time.Minute)); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if attempts < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.URL, "api", "secret")
	err := client.SendDigest(context.Background(), domain.NotificationDigest{UserID: 1, NotificationDate: time.Now().UTC()})
	if err != nil {
		t.Fatalf("SendDigest() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}
