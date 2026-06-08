package stats_service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	sponsorblock_repository "getytstatsapi/internal/entities/sponsorblock/repository"
	stats_repository "getytstatsapi/internal/entities/stats/repository"
)

type Service struct {
	repository             stats_repository.Repository
	sponsorBlockRepository sponsorblock_repository.Repository
	now                   func() time.Time
}

func New(repository stats_repository.Repository, sponsorBlockRepository sponsorblock_repository.Repository) *Service {
	return &Service{
		repository:             repository,
		sponsorBlockRepository: sponsorBlockRepository,
		now:                    time.Now,
	}
}

func (s *Service) GetVideos(ctx context.Context, query domain.StatsQuery) ([]domain.StatsVideo, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("%w: stats repository is not configured", core_errors.ErrNotFound)
	}

	videos, err := s.repository.GetVideos(ctx, query)
	if err != nil {
		return nil, err
	}

	fetchedAt := s.now().UTC()
	for idx := range videos {
		if videos[idx].ViewsUpdatedAt.IsZero() {
			videos[idx].ViewsUpdatedAt = fetchedAt
		}
	}

	if !containsColumn(query.Columns, domain.StatsColumnAdTimings) || s.sponsorBlockRepository == nil {
		return videos, nil
	}

	for idx := range videos {
		if strings.TrimSpace(videos[idx].VideoID) == "" {
			continue
		}

		segments, err := s.sponsorBlockRepository.GetSkipSegments(ctx, videos[idx].VideoID)
		if err != nil {
			return nil, fmt.Errorf("get sponsorblock segments for video %s: %w", videos[idx].VideoID, err)
		}

		videos[idx].SkipSegments = segments
	}

	return videos, nil
}

func (s *Service) BuildCSV(videos []domain.StatsVideo, columns []domain.StatsColumn) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	columns = normalizeColumns(columns)
	header := make([]string, 0, len(columns))
	for _, column := range columns {
		header = append(header, columnHeader(column))
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	var totalViews uint64
	for idx, video := range videos {
		record := make([]string, 0, len(columns))
		for _, column := range columns {
			record = append(record, columnValue(column, idx, video))
		}
		totalViews += video.Views

		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("write csv record: %w", err)
		}
	}

	viewsColumnIndex := indexOfColumn(columns, domain.StatsColumnViews)
	if viewsColumnIndex >= 0 {
		footer := make([]string, len(columns))
		footer[viewsColumnIndex] = fmt.Sprint(totalViews)
		if viewsColumnIndex > 0 {
			footer[viewsColumnIndex-1] = "Общее количество просмотров:"
		} else {
			footer[0] = "Общее количество просмотров: " + fmt.Sprint(totalViews)
		}

		if err := writer.Write(footer); err != nil {
			return nil, fmt.Errorf("write csv footer: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}

	return buf.Bytes(), nil
}

func normalizeColumns(columns []domain.StatsColumn) []domain.StatsColumn {
	return domain.NormalizeStatsColumns(columns)
}

func containsColumn(columns []domain.StatsColumn, expected domain.StatsColumn) bool {
	for _, column := range normalizeColumns(columns) {
		if column == expected {
			return true
		}
	}
	return false
}

func indexOfColumn(columns []domain.StatsColumn, expected domain.StatsColumn) int {
	for idx, column := range columns {
		if column == expected {
			return idx
		}
	}
	return -1
}

func columnHeader(column domain.StatsColumn) string {
	switch column {
	case domain.StatsColumnID:
		return "ID"
	case domain.StatsColumnPublishDate:
		return "Дата публикации"
	case domain.StatsColumnVideoURL:
		return "Ссылка на видео"
	case domain.StatsColumnViews:
		return "Кол-во просмотров"
	case domain.StatsColumnAdTimings:
		return "Тайминги рекламы"
	case domain.StatsColumnViewsUpdatedAt:
		return "Дата обновления просмотров"
	default:
		return string(column)
	}
}

func columnValue(column domain.StatsColumn, idx int, video domain.StatsVideo) string {
	switch column {
	case domain.StatsColumnID:
		return fmt.Sprint(idx + 1)
	case domain.StatsColumnPublishDate:
		return "'" + video.PublishDate.Format("2006-01-02 15:04")
	case domain.StatsColumnVideoURL:
		return video.URL
	case domain.StatsColumnViews:
		return fmt.Sprint(video.Views)
	case domain.StatsColumnAdTimings:
		return formatSponsorTimings(video.SkipSegments)
	case domain.StatsColumnViewsUpdatedAt:
		if video.ViewsUpdatedAt.IsZero() {
			return "-"
		}
		return "'" + video.ViewsUpdatedAt.Format("2006-01-02 15:04")
	default:
		return ""
	}
}

func formatSponsorTimings(segments []domain.SponsorBlockSegment) string {
	if len(segments) == 0 {
		return "-"
	}

	sponsorSegments := make([]domain.SponsorBlockSegment, 0, len(segments))
	for _, segment := range segments {
		if segment.Category == "sponsor" {
			sponsorSegments = append(sponsorSegments, segment)
		}
	}
	if len(sponsorSegments) == 0 {
		return "-"
	}

	sort.Slice(sponsorSegments, func(i, j int) bool {
		return sponsorSegments[i].StartTime < sponsorSegments[j].StartTime
	})

	parts := make([]string, 0, len(sponsorSegments))
	for _, segment := range sponsorSegments {
		parts = append(parts, formatSeconds(segment.StartTime)+"-"+formatSeconds(segment.EndTime))
	}

	return strings.Join(parts, "; ")
}

func formatSeconds(value float64) string {
	duration := time.Duration(value * float64(time.Second))
	hours := int(duration / time.Hour)
	duration -= time.Duration(hours) * time.Hour
	minutes := int(duration / time.Minute)
	duration -= time.Duration(minutes) * time.Minute
	seconds := int(duration / time.Second)
	duration -= time.Duration(seconds) * time.Second
	milliseconds := int(duration / time.Millisecond)

	if milliseconds == 0 {
		if hours > 0 {
			return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
		}
		return fmt.Sprintf("%02d:%02d", minutes, seconds)
	}

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
	}
	return fmt.Sprintf("%02d:%02d.%03d", minutes, seconds, milliseconds)
}
