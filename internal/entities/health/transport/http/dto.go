package health_http

import "getytstatsapi/internal/core/domain"

type GetHealthResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func NewGetHealthResponse(status domain.HealthStatus) GetHealthResponse {
	return GetHealthResponse{
		Name:   status.Name,
		Status: status.Status,
	}
}
