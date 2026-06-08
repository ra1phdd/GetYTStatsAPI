package postgres_repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"getytstatsapi/internal/core/domain"
	postgres_sqlc "getytstatsapi/internal/core/infra/postgres/sqlc"
)

type HistoryStore struct {
	db      *sql.DB
	queries *postgres_sqlc.Queries
	now     func() time.Time
}

func NewHistoryStore(db *sql.DB) *HistoryStore {
	return &HistoryStore{
		db:      db,
		queries: postgres_sqlc.New(db),
		now:     time.Now,
	}
}

func (s *HistoryStore) SaveFetch(ctx context.Context, query domain.StatsQuery, videos []domain.StatsVideo) error {
	if s == nil || s.db == nil || s.queries == nil {
		return nil
	}

	hiddenVideos, err := json.Marshal(query.HiddenVideos)
	if err != nil {
		return fmt.Errorf("marshal hidden videos: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	queries := s.queries.WithTx(tx)
	requestID, err := queries.CreateStatsRequest(ctx, postgres_sqlc.CreateStatsRequestParams{
		ChannelID:    query.ChannelID,
		AdWord:       query.AdWord,
		StartDate:    query.StartDate.UTC(),
		EndDate:      query.EndDate.UTC(),
		HiddenVideos: hiddenVideos,
		FetchedAt:    s.now().UTC(),
	})
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("create stats request: %w", err)
	}

	for idx, video := range videos {
		if err := queries.CreateStatsVideo(ctx, postgres_sqlc.CreateStatsVideoParams{
			RequestID:   requestID,
			Position:    int32(idx),
			Name:        video.Name,
			PublishDate: video.PublishDate.UTC(),
			Views:       int64(video.Views),
			Url:         video.URL,
		}); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("create stats video: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}
