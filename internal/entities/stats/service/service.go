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
	now                    func() time.Time
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

	return s.PopulateSponsorSegments(ctx, videos, query.Columns)
}

func (s *Service) PopulateSponsorSegments(ctx context.Context, videos []domain.StatsVideo, columns []domain.StatsColumn) ([]domain.StatsVideo, error) {
	if s == nil || !containsColumn(columns, domain.StatsColumnAdTimings) || s.sponsorBlockRepository == nil {
		return videos, nil
	}

	populated := append([]domain.StatsVideo(nil), videos...)
	for idx := range populated {
		if strings.TrimSpace(populated[idx].VideoID) == "" || len(populated[idx].SkipSegments) > 0 {
			continue
		}

		segments, err := s.sponsorBlockRepository.GetSkipSegments(ctx, populated[idx].VideoID)
		if err != nil {
			return nil, fmt.Errorf("get sponsorblock segments for video %s: %w", populated[idx].VideoID, err)
		}

		populated[idx].SkipSegments = segments
	}

	return populated, nil
}

func (s *Service) BuildCSV(videos []domain.StatsVideo, columns []domain.StatsColumn) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	columns = normalizeColumns(columns)
	viewsUpdatedAtValue := sharedViewsUpdatedAtValue(videos)
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
			record = append(record, columnValue(column, idx, video, viewsUpdatedAtValue))
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

func columnValue(column domain.StatsColumn, idx int, video domain.StatsVideo, viewsUpdatedAtValue string) string {
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
		if idx > 0 {
			return ""
		}
		return viewsUpdatedAtValue
	default:
		return ""
	}
}

func sharedViewsUpdatedAtValue(videos []domain.StatsVideo) string {
	for _, video := range videos {
		if !video.ViewsUpdatedAt.IsZero() {
			return "'" + video.ViewsUpdatedAt.Format("2006-01-02 15:04")
		}
	}
	return "-"
}

func formatSponsorTimings(segments []domain.SponsorBlockSegment) string {
	if len(segments) == 0 {
		return "-"
	}

	sponsorSegments := make([]domain.SponsorBlockSegment, 0, len(segments))
	for _, segment := range segments {
		if strings.EqualFold(strings.TrimSpace(segment.Category), "sponsor") {
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

	return strings.Join(parts, ", ")
}

func formatSeconds(value float64) string {
	totalSeconds := int(value)
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
