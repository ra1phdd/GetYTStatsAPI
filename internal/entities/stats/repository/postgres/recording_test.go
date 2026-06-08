package postgres_repository

import (
	"context"
	"testing"
	"time"

	"getytstatsapi/internal/core/domain"
)

type recordingStubRepository struct {
	calls  int
	result []domain.StatsVideo
}

func (r *recordingStubRepository) GetVideos(_ context.Context, _ domain.StatsQuery) ([]domain.StatsVideo, error) {
	r.calls++
	return append([]domain.StatsVideo(nil), r.result...), nil
}

type recordingStubStore struct {
	calls  int
	query  domain.StatsQuery
	videos []domain.StatsVideo
}

func (s *recordingStubStore) SaveFetch(_ context.Context, query domain.StatsQuery, videos []domain.StatsVideo) error {
	s.calls++
	s.query = query
	s.videos = append([]domain.StatsVideo(nil), videos...)
	return nil
}

func TestRecorderStoresFetchedVideos(t *testing.T) {
	base := &recordingStubRepository{
		result: []domain.StatsVideo{
			domain.NewStatsVideo("video", time.Date(2025, 1, 2, 3, 4, 0, 0, time.UTC), 10, "https://example.com"),
		},
	}
	store := &recordingStubStore{}
	repository := NewRecorder(base, store)

	query := domain.NewStatsQuery(
		"channel",
		"word",
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
		[]string{"hidden1"},
		nil,
	)

	videos, err := repository.GetVideos(context.Background(), query)
	if err != nil {
		t.Fatalf("GetVideos() error = %v", err)
	}

	if base.calls != 1 {
		t.Fatalf("underlying repository calls = %d, want 1", base.calls)
	}
	if store.calls != 1 {
		t.Fatalf("store calls = %d, want 1", store.calls)
	}
	if len(videos) != 1 || len(store.videos) != 1 {
		t.Fatalf("videos were not propagated correctly")
	}
	if store.query.ChannelID != query.ChannelID {
		t.Fatalf("stored query channel = %q, want %q", store.query.ChannelID, query.ChannelID)
	}
}
