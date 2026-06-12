package core_http_server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"getytstatsapi/internal/core/security/serviceauth"

	"github.com/ra1phdd/logger"
)

func TestRegisterAPIRoutersPreservesVersionedPathForSignedRequests(t *testing.T) {
	const (
		serviceID = "telegram-bot"
		secret    = "telegram-secret"
	)

	server := NewHTTPServer(":0", logger.New())
	router := NewAPIVersionRouter(ApiVersion1)
	nonces := serviceauth.NewNonceStore(time.Minute)
	router.RegisterRoutes(
		NewRoute(http.MethodGet, "/signed", func(w http.ResponseWriter, r *http.Request) {
			if err := serviceauth.VerifyRequest(r, serviceID, secret, nonces); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
		}),
	)
	server.RegisterAPIRouters(router)

	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/v1/signed", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if err := serviceauth.SignRequest(req, serviceID, secret); err != nil {
		t.Fatalf("sign request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
