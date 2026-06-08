package postgres_repository

import (
	"context"
	"fmt"

	"getytstatsapi/internal/core/domain"
	stats_repository "getytstatsapi/internal/entities/stats/repository"
)

type Recorder struct {
	next  stats_repository.Repository
	store interface {
		SaveFetch(context.Context, domain.StatsQuery, []domain.StatsVideo) error
	}
}

func NewRecorder(next stats_repository.Repository, store interface {
	SaveFetch(context.Context, domain.StatsQuery, []domain.StatsVideo) error
}) *Recorder {
	return &Recorder{
		next:  next,
		store: store,
	}
}

func (r *Recorder) GetVideos(ctx context.Context, query domain.StatsQuery) ([]domain.StatsVideo, error) {
	if r == nil || r.next == nil {
		return nil, nil
	}

	videos, err := r.next.GetVideos(ctx, query)
	if err != nil {
		return nil, err
	}

	if r.store == nil {
		return videos, nil
	}

	if err := r.store.SaveFetch(ctx, query, videos); err != nil {
		return nil, fmt.Errorf("save stats fetch history: %w", err)
	}

	return videos, nil
}
