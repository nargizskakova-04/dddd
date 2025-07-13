package v1

import (
	"encoding/json"
	"net/http"
	"strings"

	"crypto/internal/core/port"
)

type PriceHandler struct {
	priceService port.PriceService
}

func NewPriceHandler(
	priceService port.PriceService,
) *PriceHandler {
	return &PriceHandler{
		priceService: priceService,
	}
}

// Response structures
type LatestPriceResponse struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
	Exchange  string  `json:"exchange,omitempty"` // omitempty for cross-exchange responses
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Supported symbols
var supportedSymbols = map[string]bool{
	"BTCUSDT":  true,
	"DOGEUSDT": true,
	"TONUSDT":  true,
	"SOLUSDT":  true,
	"ETHUSDT":  true,
}

func (h *PriceHandler) GetLatestPrice(w http.ResponseWriter, r *http.Request) {
	// Extract symbol from URL path
	symbol := r.PathValue("symbol")
	if symbol == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "missing symbol parameter")
		return
	}

	// Normalize symbol to uppercase
	symbol = strings.ToUpper(symbol)

	// Validate symbol
	if !supportedSymbols[symbol] {
		h.writeErrorResponse(w, http.StatusBadRequest, "unsupported symbol: "+symbol)
		return
	}

	// Call service to get latest price
	marketData, err := h.priceService.GetLatestPrice(r.Context(), symbol)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to get latest price: "+err.Error())
		return
	}

	if marketData == nil {
		h.writeErrorResponse(w, http.StatusNotFound, "no price data found for symbol: "+symbol)
		return
	}

	// Prepare response
	response := LatestPriceResponse{
		Symbol:    marketData.Symbol,
		Price:     marketData.Price,
		Timestamp: marketData.Timestamp,
		Exchange:  marketData.Exchange,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *PriceHandler) GetLatestPriceByExchange(w http.ResponseWriter, r *http.Request) {
	// Extract exchange and symbol from URL path
	exchange := r.PathValue("exchange")
	symbol := r.PathValue("symbol")

	if exchange == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "missing exchange parameter")
		return
	}

	if symbol == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "missing symbol parameter")
		return
	}

	// Normalize symbol to uppercase
	symbol = strings.ToUpper(symbol)

	// Validate symbol
	if !supportedSymbols[symbol] {
		h.writeErrorResponse(w, http.StatusBadRequest, "unsupported symbol: "+symbol)
		return
	}

	// Call service to get latest price by exchange
	marketData, err := h.priceService.GetLatestPriceByExchange(r.Context(), symbol, exchange)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to get latest price: "+err.Error())
		return
	}

	if marketData == nil {
		h.writeErrorResponse(w, http.StatusNotFound, "no price data found for symbol: "+symbol+" on exchange: "+exchange)
		return
	}

	// Prepare response
	response := LatestPriceResponse{
		Symbol:    marketData.Symbol,
		Price:     marketData.Price,
		Timestamp: marketData.Timestamp,
		Exchange:  marketData.Exchange,
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *PriceHandler) GetHighestPrice(w http.ResponseWriter, r *http.Request) {
}

func (h *PriceHandler) GetHighestPriceByExchange(w http.ResponseWriter, r *http.Request) {
}

func (h *PriceHandler) GetLowestPrice(w http.ResponseWriter, r *http.Request) {
}

func (h *PriceHandler) GetLowestPriceByExchange(w http.ResponseWriter, r *http.Request) {
}

func (h *PriceHandler) GetAveragePrice(w http.ResponseWriter, r *http.Request) {
}

func (h *PriceHandler) GetAveragePriceByExchange(w http.ResponseWriter, r *http.Request) {
}

// Helper methods

func (h *PriceHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If we can't encode the response, log the error and send a simple error message
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal_error","message":"failed to encode response"}`))
	}
}

func (h *PriceHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorType := "bad_request"
	switch statusCode {
	case http.StatusNotFound:
		errorType = "not_found"
	case http.StatusInternalServerError:
		errorType = "internal_error"
	}

	response := ErrorResponse{
		Error:   errorType,
		Message: message,
	}

	h.writeJSONResponse(w, statusCode, response)
}
