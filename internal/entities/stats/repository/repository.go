package stats_repository

import (
	"context"

	"getytstatsapi/internal/core/domain"
)

type Repository interface {
	GetVideos(context.Context, domain.StatsQuery) ([]domain.StatsVideo, error)
}
