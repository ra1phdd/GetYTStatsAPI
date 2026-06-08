package health_http

import (
	"encoding/json"
	"net/http"

	core_http_response "getytstatsapi/internal/core/transport/http/response"
	health_service "getytstatsapi/internal/entities/health/service"
	"getytstatsapi/pkg/logger"
)

type Handler struct {
	log     *logger.Logger
	service *health_service.Service
}

func NewHandler(log *logger.Logger, service *health_service.Service) *Handler {
	return &Handler{
		log:     log,
		service: service,
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	responseHandler := core_http_response.NewHTTPResponseHandler(logger.FromContext(r.Context()), w)

	status, err := h.service.Status(r.Context())
	if err != nil {
		responseHandler.ErrorResponse("failed to get service health", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(NewGetHealthResponse(status)); err != nil {
		h.log.Error("failed to encode health response", logger.Err(err))
	}
}
