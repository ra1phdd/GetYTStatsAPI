package stats_service

import (
	"strings"
	"testing"
	"time"

	"getytstatsapi/internal/core/domain"
)

func TestBuildCSV(t *testing.T) {
	service := New(nil)

	data, err := service.BuildCSV([]domain.StatsVideo{
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
	})
	if err != nil {
		t.Fatalf("BuildCSV() error = %v", err)
	}

	got := string(data)
	if !strings.Contains(got, "Общее количество просмотров:,25") {
		t.Fatalf("BuildCSV() missing total views footer, got %q", got)
	}
	if !strings.Contains(got, "'2025-01-02 15:04") {
		t.Fatalf("BuildCSV() missing formatted publish date, got %q", got)
	}
}
