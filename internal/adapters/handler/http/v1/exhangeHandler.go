package v1

import (
	"context"
	"encoding/json"
	"net/http"

	"cryptomarket/internal/core/port"
)

type ExchangeHandler struct {
	exchangeService port.ExchangeService
}

func NewExchangeHandler(exchangeService port.ExchangeService) *ExchangeHandler {
	return &ExchangeHandler{
		exchangeService: exchangeService,
	}
}

type ModeResponse struct {
	Mode             string   `json:"mode"`
	AllowedExchanges []string `json:"allowed_exchanges"`
	Message          string   `json:"message"`
}

func (h *ExchangeHandler) SwitchToTestMode(w http.ResponseWriter, r *http.Request) {
	err := h.exchangeService.SwitchToTestMode(r.Context())
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to switch to test mode: "+err.Error())
		return
	}

	response := ModeResponse{
		Mode:             h.exchangeService.GetCurrentMode(),
		AllowedExchanges: h.exchangeService.GetModeExchanges(),
		Message:          "Successfully switched to test mode. API will now use data from test exchanges only.",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *ExchangeHandler) SwitchToLiveMode(w http.ResponseWriter, r *http.Request) {
	err := h.exchangeService.SwitchToLiveMode(r.Context())
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to switch to live mode: "+err.Error())
		return
	}

	response := ModeResponse{
		Mode:             h.exchangeService.GetCurrentMode(),
		AllowedExchanges: h.exchangeService.GetModeExchanges(),
		Message:          "Successfully switched to live mode. API will now use data from live exchanges only.",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *ExchangeHandler) SwitchToAllMode(w http.ResponseWriter, r *http.Request) {
	if svc, ok := h.exchangeService.(interface{ SwitchToAllMode(context.Context) error }); ok {
		err := svc.SwitchToAllMode(r.Context())
		if err != nil {
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to switch to all mode: "+err.Error())
			return
		}

		response := ModeResponse{
			Mode:             h.exchangeService.GetCurrentMode(),
			AllowedExchanges: h.exchangeService.GetModeExchanges(),
			Message:          "Successfully switched to all mode. API will now use data from all exchanges.",
		}

		h.writeJSONResponse(w, http.StatusOK, response)
	} else {
		h.writeErrorResponse(w, http.StatusNotImplemented, "All mode is not supported by this service implementation")
	}
}

func (h *ExchangeHandler) GetCurrentMode(w http.ResponseWriter, r *http.Request) {
	response := ModeResponse{
		Mode:             h.exchangeService.GetCurrentMode(),
		AllowedExchanges: h.exchangeService.GetModeExchanges(),
		Message:          "Current mode information",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *ExchangeHandler) GetServiceStats(w http.ResponseWriter, r *http.Request) {
	if svc, ok := h.exchangeService.(interface{ GetStats() map[string]interface{} }); ok {
		stats := svc.GetStats()
		h.writeJSONResponse(w, http.StatusOK, stats)
	} else {
		h.writeErrorResponse(w, http.StatusNotImplemented, "Service statistics are not available")
	}
}

func (h *ExchangeHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal_error","message":"failed to encode response"}`))
	}
}

func (h *ExchangeHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorType := "bad_request"
	switch statusCode {
	case http.StatusNotFound:
		errorType = "not_found"
	case http.StatusInternalServerError:
		errorType = "internal_error"
	case http.StatusNotImplemented:
		errorType = "not_implemented"
	}

	response := ErrorResponse{
		Error:   errorType,
		Message: message,
	}

	h.writeJSONResponse(w, statusCode, response)
}
