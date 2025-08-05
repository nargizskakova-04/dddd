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

	// Determine HTTP status code based on health
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

	// Determine HTTP status code based on overall health
	statusCode := http.StatusOK
	if status, ok := detailedHealth["overall_status"].(string); ok && status != "healthy" {
		statusCode = http.StatusServiceUnavailable
	}

	h.writeJSONResponse(w, statusCode, detailedHealth)
}

// Helper methods

func (h *HealthHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If we can't encode the response, log the error and send a simple error message
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

	// Add timestamp for health endpoints
	healthResponse := map[string]interface{}{
		"error":     response.Error,
		"message":   response.Message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"status":    "error",
	}

	h.writeJSONResponse(w, statusCode, healthResponse)
}
