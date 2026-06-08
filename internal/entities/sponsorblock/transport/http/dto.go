package sponsorblock_http

import (
	"fmt"
	"strings"

	core_errors "getytstatsapi/internal/core/errors"
	core_http_request "getytstatsapi/internal/core/transport/http/request"
	"getytstatsapi/internal/core/domain"
)

type GetSegmentsRequest struct {
	VideoID string `validate:"required"`
}

type GetSegmentsResponse struct {
	VideoID  string                       `json:"video_id"`
	Segments []GetSegmentsResponseSegment `json:"segments"`
}

type GetSegmentsResponseSegment struct {
	UUID          string  `json:"uuid"`
	Category      string  `json:"category"`
	ActionType    string  `json:"action_type"`
	StartTime     float64 `json:"start_time"`
	EndTime       float64 `json:"end_time"`
	VideoDuration float64 `json:"video_duration"`
	Locked        int     `json:"locked"`
	Votes         int     `json:"votes"`
	Description   string  `json:"description"`
}

func NewGetSegmentsRequestFromQuery(query map[string][]string) GetSegmentsRequest {
	return GetSegmentsRequest{VideoID: firstQueryValue(query, "video_id")}
}

func (r GetSegmentsRequest) Validate() (string, error) {
	if err := core_http_request.ValidateRequest(r); err != nil {
		return "", fmt.Errorf("%w: %w", core_errors.ErrInvalidArgument, err)
	}

	return strings.TrimSpace(r.VideoID), nil
}

func NewGetSegmentsResponse(videoID string, segments []domain.SponsorBlockSegment) GetSegmentsResponse {
	responseSegments := make([]GetSegmentsResponseSegment, 0, len(segments))
	for _, segment := range segments {
		responseSegments = append(responseSegments, GetSegmentsResponseSegment{
			UUID:          segment.UUID,
			Category:      segment.Category,
			ActionType:    segment.ActionType,
			StartTime:     segment.StartTime,
			EndTime:       segment.EndTime,
			VideoDuration: segment.VideoDuration,
			Locked:        segment.Locked,
			Votes:         segment.Votes,
			Description:   segment.Description,
		})
	}

	return GetSegmentsResponse{
		VideoID:  videoID,
		Segments: responseSegments,
	}
}

func firstQueryValue(query map[string][]string, key string) string {
	values := query[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
