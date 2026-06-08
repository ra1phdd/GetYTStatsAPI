package stats_service

import (
	"strings"
	"testing"
	"time"

	"getytstatsapi/internal/core/domain"
)

func TestBuildCSV(t *testing.T) {
	service := New(nil, nil)

	videos := []domain.StatsVideo{
		domain.NewStatsVideo(
			"video one",
			time.Date(2025, time.January, 2, 15, 4, 0, 0, time.UTC),
			10,
			"https://example.com/1",
		),
		domain.NewStatsVideo(
			"video two",
			time.Date(2025, time.January, 3, 16, 5, 0, 0, time.UTC),
			15,
			"https://example.com/2",
		),
	}
	videos[0].ViewsUpdatedAt = time.Date(2025, time.January, 5, 10, 30, 0, 0, time.UTC)
	videos[0].SkipSegments = []domain.SponsorBlockSegment{
		domain.NewSponsorBlockSegment("uuid-1", "sponsor", "skip", 5.5, 18.25, 120, 0, 10, ""),
	}

	data, err := service.BuildCSV(videos, []domain.StatsColumn{
		domain.StatsColumnID,
		domain.StatsColumnPublishDate,
		domain.StatsColumnVideoURL,
		domain.StatsColumnViews,
		domain.StatsColumnAdTimings,
		domain.StatsColumnViewsUpdatedAt,
	})
	if err != nil {
		t.Fatalf("BuildCSV() error = %v", err)
	}

	got := string(data)
	if !strings.Contains(got, "Общее количество просмотров:") || !strings.Contains(got, ",25,") {
		t.Fatalf("BuildCSV() missing total views footer, got %q", got)
	}
	if !strings.Contains(got, "'2025-01-02 15:04") {
		t.Fatalf("BuildCSV() missing formatted publish date, got %q", got)
	}
	if !strings.Contains(got, "00:05.500-00:18.250") {
		t.Fatalf("BuildCSV() missing ad timings, got %q", got)
	}
	if !strings.Contains(got, "Дата обновления просмотров") {
		t.Fatalf("BuildCSV() missing custom header, got %q", got)
	}
}
