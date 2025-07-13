package v1

import (
	"net/http"

	"crypto/internal/core/port"
)

type HealthHandler struct {
	healthService port.HealthService
}

func NewHealthHandler(
	healthService port.HealthService,
) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
	}
}

func (h *HealthHandler) GetSystemHealth(w http.ResponseWriter, r *http.Request) {
}

func (h *HealthHandler) GetDetailedHealth(w http.ResponseWriter, r *http.Request) {
}
