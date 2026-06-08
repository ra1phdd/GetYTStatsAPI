package sponsorblock_http

import (
	"encoding/json"
	"net/http"

	core_http_response "getytstatsapi/internal/core/transport/http/response"
	sponsorblock_service "getytstatsapi/internal/entities/sponsorblock/service"
	"getytstatsapi/pkg/logger"
)

type Handler struct {
	log     *logger.Logger
	service *sponsorblock_service.Service
}

func NewHandler(log *logger.Logger, service *sponsorblock_service.Service) *Handler {
	return &Handler{
		log:     log,
		service: service,
	}
}

func (h *Handler) GetSegments(w http.ResponseWriter, r *http.Request) {
	responseHandler := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)

	request := NewGetSegmentsRequestFromQuery(r.URL.Query())
	videoID, err := request.Validate()
	if err != nil {
		responseHandler.ErrorResponse("invalid sponsorblock request", err)
		return
	}

	segments, err := h.service.GetSkipSegments(r.Context(), videoID)
	if err != nil {
		responseHandler.ErrorResponse("failed to get sponsorblock segments", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(NewGetSegmentsResponse(videoID, segments)); err != nil {
		h.log.Error("failed to encode sponsorblock response", logger.Err(err))
	}
}
