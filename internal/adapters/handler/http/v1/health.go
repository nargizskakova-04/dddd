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

func NewHealthHandler(healthService port.HealthService) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
	}
}

func (h *HealthHandler) GetSystemHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.healthService == nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "health service unavailable")
		return
	}

	healthData := h.healthService.GetSystemHealth(r.Context())

	h.writeJSONResponse(w, http.StatusOK, healthData)
}

func (h *HealthHandler) GetDetailedHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.healthService == nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "health service unavailable")
		return
	}

	healthData := h.healthService.GetSystemHealth(r.Context())

	// Add detailed information
	detailedHealth := map[string]interface{}{
		"status":    "healthy", // Default to healthy
		"timestamp": time.Now().Unix(),
		"services":  healthData,
	}

	// Determine overall status
	for _, service := range healthData {
		if serviceData, ok := service.(map[string]interface{}); ok {
			if status, exists := serviceData["status"]; exists && status != "healthy" {
				detailedHealth["status"] = "unhealthy"
				break
			}
		}
	}

	h.writeJSONResponse(w, http.StatusOK, detailedHealth)
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
	errorType := "bad_request"
	switch statusCode {
	case http.StatusNotFound:
		errorType = "not_found"
	case http.StatusInternalServerError:
		errorType = "internal_error"
	case http.StatusMethodNotAllowed:
		errorType = "method_not_allowed"
	}

	response := ErrorResponse{
		Error:   errorType,
		Message: message,
	}

	h.writeJSONResponse(w, statusCode, response)
}
