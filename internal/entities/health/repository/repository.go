package health_repository

import (
	"context"

	"getytstatsapi/internal/core/domain"
)

type Repository interface {
	Status(context.Context) (domain.HealthStatus, error)
}
