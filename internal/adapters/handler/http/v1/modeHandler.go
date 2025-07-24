package v1

import (
	"context"
	"net/http"

	"cryptomarket/internal/core/port"
)

type ExchangeHandler struct {
	exchangeService port.ExchangeService
	ctx             context.Context
}

func NewExchangeHandler(
	exchangeService port.ExchangeService,
) *ExchangeHandler {
	return &ExchangeHandler{
		exchangeService: exchangeService,
	}
}

func (h *ExchangeHandler) SwitchToTestExchange(w http.ResponseWriter, r *http.Request) {
	h.exchangeService.SwitchToTestMode(h.ctx)
}

func (h *ExchangeHandler) SwitchToLiveExchange(w http.ResponseWriter, r *http.Request) {
	h.exchangeService.SwitchToLiveMode(h.ctx)
}

func (h *ExchangeHandler) GetCurrentExchange(w http.ResponseWriter, r *http.Request) {
}
