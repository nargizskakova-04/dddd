package v1

import (
	"encoding/json"
	"net/http"

	"cryptomarket/internal/core/port"
)

type ExchangeHandler struct {
	exchangeService port.ExchangeService
}

func NewExchangeHandler(
	exchangeService port.ExchangeService,
) *ExchangeHandler {
	return &ExchangeHandler{
		exchangeService: exchangeService,
	}
}

func (h *ExchangeHandler) SwitchToTestMode(w http.ResponseWriter, r *http.Request) {
	h.exchangeService.SwitchToTestMode(r.Context())
}

func (h *ExchangeHandler) SwitchToLiveMode(w http.ResponseWriter, r *http.Request) {
	h.exchangeService.SwitchToLiveMode(r.Context())
}

func (h *ExchangeHandler) GetCurrentMode(w http.ResponseWriter, r *http.Request) {
	h.writeJSONResponse(w, http.StatusOK, h.exchangeService.GetCurrentMode())
}

func (h *ExchangeHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If we can't encode the response, log the error and send a simple error message
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal_error","message":"failed to encode response"}`))
	}
}
