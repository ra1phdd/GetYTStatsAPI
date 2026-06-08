package sponsorblock_service

import (
	"context"
	"fmt"
	"strings"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	sponsorblock_repository "getytstatsapi/internal/entities/sponsorblock/repository"
)

type Service struct {
	repository sponsorblock_repository.Repository
}

func New(repository sponsorblock_repository.Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetSkipSegments(ctx context.Context, videoID string) ([]domain.SponsorBlockSegment, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("%w: sponsorblock repository is not configured", core_errors.ErrNotFound)
	}

	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return nil, fmt.Errorf("%w: video id is required", core_errors.ErrInvalidArgument)
	}

	return s.repository.GetSkipSegments(ctx, videoID)
}
