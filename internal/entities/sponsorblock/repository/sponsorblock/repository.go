package sponsorblock_repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
)

const defaultBaseURL = "https://sponsor.ajay.app"

var skipCategories = []string{
	"sponsor",
	"intro",
	"outro",
	"selfpromo",
	"interaction",
	"preview",
	"music_offtopic",
	"filler",
	"exclusive_access",
}

type Repository struct {
	baseURL string
	client  *http.Client
}

type skipSegmentResponse struct {
	Segment       []float64 `json:"segment"`
	UUID          string    `json:"UUID"`
	Category      string    `json:"category"`
	VideoDuration float64   `json:"videoDuration"`
	ActionType    string    `json:"actionType"`
	Locked        int       `json:"locked"`
	Votes         int       `json:"votes"`
	Description   string    `json:"description"`
}

func New(baseURL string) *Repository {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	return &Repository{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (r *Repository) GetSkipSegments(ctx context.Context, videoID string) ([]domain.SponsorBlockSegment, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" {
		return nil, fmt.Errorf("%w: video id is required", core_errors.ErrInvalidArgument)
	}

	endpoint, err := url.Parse(r.baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse sponsorblock base url: %w", err)
	}
	endpoint.Path = path.Join(endpoint.Path, "/api/skipSegments")

	query := endpoint.Query()
	query.Set("videoID", videoID)
	query.Set("actionType", "skip")
	query.Set("service", "YouTube")
	for _, category := range skipCategories {
		query.Add("category", category)
	}
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build sponsorblock request: %w", err)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request sponsorblock segments: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return []domain.SponsorBlockSegment{}, nil
	case http.StatusBadRequest:
		return nil, fmt.Errorf("%w: sponsorblock rejected request for video %s", core_errors.ErrInvalidArgument, videoID)
	default:
		return nil, fmt.Errorf("unexpected sponsorblock status %d", resp.StatusCode)
	}

	var payload []skipSegmentResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode sponsorblock response: %w", err)
	}

	segments := make([]domain.SponsorBlockSegment, 0, len(payload))
	for _, item := range payload {
		if len(item.Segment) != 2 {
			continue
		}

		segments = append(segments, domain.NewSponsorBlockSegment(
			item.UUID,
			item.Category,
			item.ActionType,
			item.Segment[0],
			item.Segment[1],
			item.VideoDuration,
			item.Locked,
			item.Votes,
			item.Description,
		))
	}

	return segments, nil
}
