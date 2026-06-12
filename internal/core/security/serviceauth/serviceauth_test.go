package serviceauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSignAndVerifyRequest(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodPost, "http://example.com/internal/test?x=1", http.NoBody)
	if err := SignRequest(req, "api", "secret"); err != nil {
		t.Fatalf("SignRequest() error = %v", err)
	}
	store := NewNonceStore(time.Minute)
	if err := VerifyRequest(req, "api", "secret", store); err != nil {
		t.Fatalf("VerifyRequest() error = %v", err)
	}
	if err := VerifyRequest(req, "api", "secret", store); err == nil {
		t.Fatal("VerifyRequest() second call error = nil, want nonce replay error")
	}

	store.Forget("api:" + req.Header.Get(HeaderNonce))
	if err := VerifyRequest(req, "api", "secret", store); err != nil {
		t.Fatalf("VerifyRequest() after Forget error = %v", err)
	}
}
