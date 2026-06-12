package stats_http

import (
	"net/http"
	"time"

	core_http_response "getytstatsapi/internal/core/transport/http/response"
	stats_service "getytstatsapi/internal/entities/stats/service"
	"github.com/ra1phdd/logger"
)

type Handler struct {
	log     *logger.Logger
	service *stats_service.Service
}

func NewHandler(log *logger.Logger, service *stats_service.Service) *Handler {
	return &Handler{
		log:     log,
		service: service,
	}
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	responseHandler := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)

	request := NewGetStatsRequestFromQuery(r.URL.Query())
	query, err := request.ToDomain(time.Now())
	if err != nil {
		responseHandler.ErrorResponse("invalid stats request", err)
		return
	}

	videos, err := h.service.GetVideos(r.Context(), query)
	if err != nil {
		responseHandler.ErrorResponse("failed to get videos", err)
		return
	}

	data, err := h.service.BuildCSV(videos, query.Columns)
	if err != nil {
		responseHandler.ErrorResponse("failed to build csv", err)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=stats.csv")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		h.log.Error("failed to write stats csv", logger.Err(err))
	}
}
