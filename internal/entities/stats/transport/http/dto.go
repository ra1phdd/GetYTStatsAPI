package stats_http

import (
	"fmt"
	"strings"
	"time"

	"getytstatsapi/internal/core/domain"
	core_errors "getytstatsapi/internal/core/errors"
	core_http_request "getytstatsapi/internal/core/transport/http/request"
)

const dateLayout = "2006-01-02"

type GetStatsRequest struct {
	ChannelID    string `validate:"required"`
	AdWord       string `validate:"required"`
	StartDateRaw string `validate:"required,datetime=2006-01-02"`
	EndDateRaw   string `validate:"omitempty,datetime=2006-01-02"`
	HiddenVideos string
}

func NewGetStatsRequestFromQuery(query map[string][]string) GetStatsRequest {
	return GetStatsRequest{
		ChannelID:    firstQueryValue(query, "channel_id"),
		AdWord:       firstQueryValue(query, "ad_word"),
		StartDateRaw: firstQueryValue(query, "start_date"),
		EndDateRaw:   firstQueryValue(query, "end_date"),
		HiddenVideos: firstQueryValue(query, "hidden_videos"),
	}
}

func (r GetStatsRequest) ToDomain(now time.Time) (domain.StatsQuery, error) {
	if err := core_http_request.ValidateRequest(r); err != nil {
		return domain.StatsQuery{}, fmt.Errorf("%w: %w", core_errors.ErrInvalidArgument, err)
	}

	startDate, err := time.Parse(dateLayout, strings.TrimSpace(r.StartDateRaw))
	if err != nil {
		return domain.StatsQuery{}, fmt.Errorf("%w: invalid start_date", core_errors.ErrInvalidArgument)
	}

	endDate := now
	if strings.TrimSpace(r.EndDateRaw) != "" {
		endDate, err = time.Parse(dateLayout, strings.TrimSpace(r.EndDateRaw))
		if err != nil {
			return domain.StatsQuery{}, fmt.Errorf("%w: invalid end_date", core_errors.ErrInvalidArgument)
		}
	}

	if endDate.Before(startDate) {
		return domain.StatsQuery{}, fmt.Errorf("%w: end_date must not be before start_date", core_errors.ErrInvalidArgument)
	}

	return domain.NewStatsQuery(
		strings.TrimSpace(r.ChannelID),
		strings.TrimSpace(r.AdWord),
		startDate,
		endDate,
		parseHiddenVideos(r.HiddenVideos),
	), nil
}

func parseHiddenVideos(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	items := strings.Split(raw, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func firstQueryValue(query map[string][]string, key string) string {
	values := query[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
