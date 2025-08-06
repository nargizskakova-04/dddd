// internal/adapters/handler/http/v1/aggregationHandler.go
package v1

import (
	"encoding/json"
	"net/http"
	"time"

	"cryptomarket/internal/core/service/aggregation"
)

type AggregationHandler struct {
	aggregationService *aggregation.AggregationService
}

func NewAggregationHandler(aggregationService *aggregation.AggregationService) *AggregationHandler {
	return &AggregationHandler{
		aggregationService: aggregationService,
	}
}

// Response structures
type AggregationStatusResponse struct {
	Status    string                 `json:"status"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

type TriggerAggregationRequest struct {
	AggregationTime string `json:"aggregation_time,omitempty"` // ISO 8601 format
}

type TriggerAggregationResponse struct {
	Status          string `json:"status"`
	Message         string `json:"message"`
	AggregationTime string `json:"aggregation_time"`
	Timestamp       string `json:"timestamp"`
}

// GetAggregationStatus returns the current status of the aggregation service
func (h *AggregationHandler) GetAggregationStatus(w http.ResponseWriter, r *http.Request) {
	if h.aggregationService == nil {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, "Aggregation service is not available")
		return
	}

	healthStatus := h.aggregationService.GetHealthStatus(r.Context())

	// Determine overall status
	status := "healthy"
	message := "Aggregation service is running normally"

	if cacheHealthy, ok := healthStatus["cache_healthy"].(bool); ok && !cacheHealthy {
		status = "degraded"
		message = "Cache is not healthy"
	}

	if repoHealthy, ok := healthStatus["repository_healthy"].(bool); ok && !repoHealthy {
		status = "degraded"
		if status == "degraded" {
			message = "Cache and repository are not healthy"
		} else {
			message = "Repository is not healthy"
		}
	}

	response := AggregationStatusResponse{
		Status:    status,
		Message:   message,
		Details:   healthStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// TriggerManualAggregation triggers a manual aggregation for a specific time or current time
func (h *AggregationHandler) TriggerManualAggregation(w http.ResponseWriter, r *http.Request) {
	if h.aggregationService == nil {
		h.writeErrorResponse(w, http.StatusServiceUnavailable, "Aggregation service is not available")
		return
	}

	// Parse request body if provided
	var req TriggerAggregationRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body: "+err.Error())
			return
		}
	}

	// Determine aggregation time
	var aggregationTime time.Time
	var err error

	if req.AggregationTime != "" {
		// Parse provided time
		aggregationTime, err = time.Parse(time.RFC3339, req.AggregationTime)
		if err != nil {
			h.writeErrorResponse(w, http.StatusBadRequest, "Invalid aggregation_time format. Use ISO 8601 format (e.g., 2023-12-25T10:30:00Z)")
			return
		}
	} else {
		// Use current time truncated to the minute
		aggregationTime = time.Now().Truncate(time.Minute)
	}

	// Trigger manual aggregation
	if err := h.aggregationService.TriggerManualAggregation(r.Context(), aggregationTime); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to trigger aggregation: "+err.Error())
		return
	}

	response := TriggerAggregationResponse{
		Status:          "success",
		Message:         "Manual aggregation completed successfully",
		AggregationTime: aggregationTime.UTC().Format(time.RFC3339),
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// GetAggregationHealth returns basic health check information
func (h *AggregationHandler) GetAggregationHealth(w http.ResponseWriter, r *http.Request) {
	if h.aggregationService == nil {
		response := map[string]interface{}{
			"status":    "unavailable",
			"message":   "Aggregation service is not available",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		h.writeJSONResponse(w, http.StatusServiceUnavailable, response)
		return
	}

	healthStatus := h.aggregationService.GetHealthStatus(r.Context())

	response := map[string]interface{}{
		"status":    "available",
		"message":   "Aggregation service is available",
		"health":    healthStatus,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// Helper methods

func (h *AggregationHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If we can't encode the response, log the error and send a simple error message
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal_error","message":"failed to encode response"}`))
	}
}

func (h *AggregationHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorType := "bad_request"
	switch statusCode {
	case http.StatusNotFound:
		errorType = "not_found"
	case http.StatusInternalServerError:
		errorType = "internal_error"
	case http.StatusServiceUnavailable:
		errorType = "service_unavailable"
	}

	response := ErrorResponse{
		Error:   errorType,
		Message: message,
	}

	h.writeJSONResponse(w, statusCode, response)
}
