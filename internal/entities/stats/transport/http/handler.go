package stats_http

import (
	"net/http"
	"strings"
	"time"

	core_config "getytstatsapi/internal/core/infra/config"
	core_http_response "getytstatsapi/internal/core/transport/http/response"
	stats_service "getytstatsapi/internal/entities/stats/service"
	"getytstatsapi/pkg/logger"
)

type Handler struct {
	baseURL string
	log     *logger.Logger
	service *stats_service.Service
}

func NewHandler(baseURL string, log *logger.Logger, service *stats_service.Service) *Handler {
	return &Handler{
		baseURL: strings.TrimSpace(baseURL),
		log:     log,
		service: service,
	}
}

func NewFromConfig(cfg *core_config.Config, log *logger.Logger, service *stats_service.Service) *Handler {
	baseURL := ""
	if cfg != nil {
		baseURL = cfg.HTTP.BaseURL
	}
	return NewHandler(baseURL, log, service)
}

func (h *Handler) GetCommand(w http.ResponseWriter, r *http.Request) {
	responseHandler := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)

	command, err := h.service.BuildCommand(h.resolveBaseURL(r), r.URL.RawQuery)
	if err != nil {
		responseHandler.ErrorResponse("failed to build import formula", err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(command))
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

	data, err := h.service.BuildCSV(videos)
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

func (h *Handler) resolveBaseURL(r *http.Request) string {
	if h.baseURL != "" {
		return strings.TrimRight(h.baseURL, "/")
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = forwardedProto
	}

	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = strings.TrimSpace(r.Header.Get("Host"))
	}

	return strings.TrimRight(scheme+"://"+host, "/")
}
