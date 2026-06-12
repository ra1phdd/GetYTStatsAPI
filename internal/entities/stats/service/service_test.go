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
	if strings.Contains(got, "Общее количество просмотров:") || !strings.Contains(got, ",25,") {
		t.Fatalf("BuildCSV() missing total views footer, got %q", got)
	}
	if !strings.Contains(got, "'2025-01-02 15:04") {
		t.Fatalf("BuildCSV() missing formatted publish date, got %q", got)
	}
	if !strings.Contains(got, "00:05-00:18") {
		t.Fatalf("BuildCSV() missing ad timings, got %q", got)
	}
	if !strings.Contains(got, "Дата обновления просмотров") {
		t.Fatalf("BuildCSV() missing custom header, got %q", got)
	}
	if strings.Count(got, "'2025-01-05 10:30") != 1 {
		t.Fatalf("BuildCSV() duplicated views updated at value, got %q", got)
	}
}

func TestFormatSponsorTimingsUsesClockFormat(t *testing.T) {
	t.Parallel()

	got := formatSponsorTimings([]domain.SponsorBlockSegment{
		domain.NewSponsorBlockSegment("intro", "intro", "skip", 10, 20, 5000, 0, 0, ""),
		domain.NewSponsorBlockSegment("s1", "sponsor", "skip", 137.735, 161.146, 3553.561, 0, 2, ""),
		domain.NewSponsorBlockSegment("s2", "sponsor", "skip", 3661.9, 3725.2, 4000, 0, 1, ""),
	})

	if got != "02:17-02:41, 01:01:01-01:02:05" {
		t.Fatalf("formatSponsorTimings() = %q", got)
	}
}
