package sponsorblock_repository

import (
	"context"

	"getytstatsapi/internal/core/domain"
)

type Repository interface {
	GetSkipSegments(context.Context, string) ([]domain.SponsorBlockSegment, error)
}
