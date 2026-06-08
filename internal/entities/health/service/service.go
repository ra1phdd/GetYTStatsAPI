package health_service

import (
	"context"
	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	health_repository "getytstatsapi/internal/entities/health/repository"
)

type Service struct {
	repository health_repository.Repository
}

func New(repository health_repository.Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Status(ctx context.Context) (domain.HealthStatus, error) {
	if s == nil || s.repository == nil {
		return domain.HealthStatus{}, core_errors.ErrNotFound
	}

	return s.repository.Status(ctx)
}
