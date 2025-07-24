package v1

import (
	"context"
	"net/http"
	"time"

	"cryptomarket/internal/core/port"
)

type ExchangeHandler struct {
	exchangeService port.ExchangeService
	ctx             context.Context
}

// ✅ ИСПРАВЛЕНО: Добавляем контекст в конструктор
func NewExchangeHandler(
	exchangeService port.ExchangeService,
	ctx context.Context,
) *ExchangeHandler {
	return &ExchangeHandler{
		exchangeService: exchangeService,
		ctx:             ctx,
	}
}

// ✅ ИСПРАВЛЕНО: Полная обработка ошибок и ответов
func (h *ExchangeHandler) SwitchToTestExchange(w http.ResponseWriter, r *http.Request) {
	if h.exchangeService == nil {
		http.Error(w, "Exchange service not available", http.StatusServiceUnavailable)
		return
	}

	// Используем контекст с таймаутом
	ctx, cancel := context.WithTimeout(h.ctx, 30*time.Second)
	defer cancel()

	err := h.exchangeService.SwitchToTestMode(ctx)
	if err != nil {
		http.Error(w, "Failed to switch to test mode: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"switched to test mode"}`))
}

// ✅ ИСПРАВЛЕНО: Полная обработка ошибок и ответов
func (h *ExchangeHandler) SwitchToLiveExchange(w http.ResponseWriter, r *http.Request) {
	if h.exchangeService == nil {
		http.Error(w, "Exchange service not available", http.StatusServiceUnavailable)
		return
	}

	// Используем контекст с таймаутом
	ctx, cancel := context.WithTimeout(h.ctx, 30*time.Second)
	defer cancel()

	err := h.exchangeService.SwitchToLiveMode(ctx)
	if err != nil {
		http.Error(w, "Failed to switch to live mode: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"switched to live mode"}`))
}

func (h *ExchangeHandler) GetCurrentExchange(w http.ResponseWriter, r *http.Request) {
	// if h.exchangeService == nil {
	// 	http.Error(w, "Exchange service not available", http.StatusServiceUnavailable)
	// 	return
	// }

	// mode := h.exchangeService.GetCurrentMode()
	// stats := h.exchangeService.GetStats()

	// // Преобразуем adapter_names в строку для JSON
	// adapterNames := ""
	// if names, ok := stats["adapter_names"].([]string); ok {
	// 	adapterNames = fmt.Sprintf("[%s]", strings.Join(names, ", "))
	// }

	// response := fmt.Sprintf(`{
	// 	"current_mode": "%s",
	// 	"is_running": %v,
	// 	"active_adapters": %v,
	// 	"healthy_adapters": %v,
	// 	"adapter_names": %s,
	// 	"aggregated_buffer": %v,
	// 	"result_buffer": %v,
	// 	"timestamp": "%s"
	// }`,
	// 	mode,
	// 	stats["is_running"],
	// 	stats["active_adapters"],
	// 	stats["healthy_adapters"],
	// 	adapterNames,
	// 	stats["aggregated_buffer"],
	// 	stats["result_buffer"],
	// 	time.Now().Format(time.RFC3339),
	// )

	// w.Header().Set("Content-Type", "application/json")
	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte(response))
}
