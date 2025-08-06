package v1

import (
	"encoding/json"
	"net/http"
	"time"

	"cryptomarket/internal/core/port"
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
	if h.healthService == nil {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, "Health service is not available")
		return
	}

	healthStatus := h.healthService.GetSystemHealth(r.Context())

	statusCode := http.StatusOK
	if status, ok := healthStatus["status"].(string); ok && status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	h.writeJSONResponse(w, statusCode, healthStatus)
}

func (h *HealthHandler) GetDetailedHealth(w http.ResponseWriter, r *http.Request) {
	if h.healthService == nil {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, "Health service is not available")
		return
	}

	detailedHealth := h.healthService.GetDetailedHealth(r.Context())

	statusCode := http.StatusOK
	if status, ok := detailedHealth["overall_status"].(string); ok && status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	h.writeJSONResponse(w, statusCode, detailedHealth)
}

func (h *HealthHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal_error","message":"failed to encode response"}`))
	}
}

func (h *HealthHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorType := "service_unavailable"
	if statusCode == http.StatusInternalServerError {
		errorType = "internal_error"
	}

	response := ErrorResponse{
		Error:   errorType,
		Message: message,
	}

	healthResponse := map[string]interface{}{
		"error":     response.Error,
		"message":   response.Message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"status":    "error",
	}

	h.writeJSONResponse(w, statusCode, healthResponse)
}
