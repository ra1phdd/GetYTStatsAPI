package stats_service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strings"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	stats_repository "getytstatsapi/internal/entities/stats/repository"
)

type Service struct {
	repository stats_repository.Repository
}

func New(repository stats_repository.Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) BuildCommand(baseURL string, rawQuery string) (string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("%w: base url is required", core_errors.ErrInvalidArgument)
	}

	return fmt.Sprintf("=IMPORTDATA(\"%s/v1/stats/get?%s\";\",\";\"en_US\")", baseURL, rawQuery), nil
}

func (s *Service) GetVideos(ctx context.Context, query domain.StatsQuery) ([]domain.StatsVideo, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("%w: stats repository is not configured", core_errors.ErrNotFound)
	}

	return s.repository.GetVideos(ctx, query)
}

func (s *Service) BuildCSV(videos []domain.StatsVideo) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{"ID", "Дата", "Название", "Просмотры", "URL"}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	var totalViews uint64
	for idx, video := range videos {
		record := []string{
			fmt.Sprint(idx + 1),
			"'" + video.PublishDate.Format("2006-01-02 15:04"),
			video.Name,
			fmt.Sprint(video.Views),
			video.URL,
		}
		totalViews += video.Views

		if err := writer.Write(record); err != nil {
			return nil, fmt.Errorf("write csv record: %w", err)
		}
	}

	footer := []string{"", "", "Общее количество просмотров:", fmt.Sprint(totalViews), ""}
	if err := writer.Write(footer); err != nil {
		return nil, fmt.Errorf("write csv footer: %w", err)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}

	return buf.Bytes(), nil
}
