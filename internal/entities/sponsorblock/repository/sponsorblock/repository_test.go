package sponsorblock_repository

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSkipSegments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("videoID"); got != "video-123" {
			t.Fatalf("videoID = %q, want %q", got, "video-123")
		}
		if got := r.URL.Query().Get("actionType"); got != "skip" {
			t.Fatalf("actionType = %q, want %q", got, "skip")
		}

		categories := r.URL.Query()["category"]
		if len(categories) == 0 {
			t.Fatal("expected skip categories in request")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"segment":[5.5,18.25],"UUID":"seg-1","category":"sponsor","videoDuration":120.0,"actionType":"skip","locked":1,"votes":10,"description":""},
			{"segment":[25,30],"UUID":"seg-2","category":"intro","videoDuration":120.0,"actionType":"skip","locked":0,"votes":4,"description":""}
		]`))
	}))
	defer server.Close()

	repository := New(server.URL)

	segments, err := repository.GetSkipSegments(context.Background(), "video-123")
	if err != nil {
		t.Fatalf("GetSkipSegments() error = %v", err)
	}

	if len(segments) != 2 {
		t.Fatalf("len(GetSkipSegments()) = %d, want 2", len(segments))
	}

	if segments[0].StartTime != 5.5 || segments[0].EndTime != 18.25 {
		t.Fatalf("first segment times = [%v, %v], want [5.5, 18.25]", segments[0].StartTime, segments[0].EndTime)
	}

	if segments[1].Category != "intro" {
		t.Fatalf("second segment category = %q, want %q", segments[1].Category, "intro")
	}
}

func TestGetSkipSegmentsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	repository := New(server.URL)

	segments, err := repository.GetSkipSegments(context.Background(), "video-123")
	if err != nil {
		t.Fatalf("GetSkipSegments() error = %v", err)
	}

	if len(segments) != 0 {
		t.Fatalf("len(GetSkipSegments()) = %d, want 0", len(segments))
	}
}
