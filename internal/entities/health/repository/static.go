package health_repository

import (
	"context"

	"getytstatsapi/internal/core/domain"
)

type StaticRepository struct {
	name string
}

func NewStaticRepository(name string) *StaticRepository {
	if name == "" {
		name = "getytstatsapi"
	}

	return &StaticRepository{name: name}
}

func (r *StaticRepository) Status(context.Context) (domain.HealthStatus, error) {
	return domain.NewHealthStatus(r.name, "ok"), nil
}
