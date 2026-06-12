package jwtutil

import "testing"

func TestSignAndParseHS256(t *testing.T) {
	t.Parallel()

	type claims struct {
		Sub string `json:"sub"`
	}

	token, err := SignHS256(claims{Sub: "123"}, "secret")
	if err != nil {
		t.Fatalf("SignHS256() error = %v", err)
	}

	var parsed claims
	if err := ParseHS256(token, "secret", &parsed); err != nil {
		t.Fatalf("ParseHS256() error = %v", err)
	}
	if parsed.Sub != "123" {
		t.Fatalf("parsed sub = %q, want 123", parsed.Sub)
	}
	if err := ParseHS256(token, "other-secret", &parsed); err == nil {
		t.Fatal("ParseHS256() with wrong secret error = nil, want error")
	}
}
