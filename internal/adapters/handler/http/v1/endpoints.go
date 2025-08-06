package v1

import "net/http"

func SetMarketRoutes(router *http.ServeMux, marketHandler *PriceHandler, healthHandler *HealthHandler, exchangeHandler *ExchangeHandler, aggregationHandler *AggregationHandler) {

	setPriceRoutes(marketHandler, router)

	setModeRoutes(exchangeHandler, router)

	setHealthRoutes(healthHandler, router)

	setAggregationRoutes(aggregationHandler, router)
}

func setPriceRoutes(handler *PriceHandler, router *http.ServeMux) {

	router.HandleFunc("GET /prices/latest/{symbol}", handler.GetLatestPrice)
	router.HandleFunc("GET /prices/latest/{exchange}/{symbol}", handler.GetLatestPriceByExchange)

	router.HandleFunc("GET /prices/highest/{symbol}", handler.GetHighestPrice)
	router.HandleFunc("GET /prices/highest/{exchange}/{symbol}", handler.GetHighestPriceByExchange)

	router.HandleFunc("GET /prices/lowest/{symbol}", handler.GetLowestPrice)
	router.HandleFunc("GET /prices/lowest/{exchange}/{symbol}", handler.GetLowestPriceByExchange)

	router.HandleFunc("GET /prices/average/{symbol}", handler.GetAveragePrice)
	router.HandleFunc("GET /prices/average/{exchange}/{symbol}", handler.GetAveragePriceByExchange)

	router.HandleFunc("GET /prices/period-info", handler.GetPeriodInfo)
}

func setModeRoutes(handler *ExchangeHandler, router *http.ServeMux) {

	router.HandleFunc("POST /mode/test", handler.SwitchToTestMode)
	router.HandleFunc("POST /mode/live", handler.SwitchToLiveMode)
	router.HandleFunc("POST /mode/all", handler.SwitchToAllMode)

	router.HandleFunc("GET /mode/current", handler.GetCurrentMode)
	router.HandleFunc("GET /mode/stats", handler.GetServiceStats)
}

func setHealthRoutes(handler *HealthHandler, router *http.ServeMux) {
	router.HandleFunc("GET /health", handler.GetSystemHealth)
	router.HandleFunc("GET /health/detailed", handler.GetDetailedHealth)
}

func setAggregationRoutes(handler *AggregationHandler, router *http.ServeMux) {

	if handler == nil {
		return
	}

	router.HandleFunc("GET /aggregation/status", handler.GetAggregationStatus)
	router.HandleFunc("GET /aggregation/health", handler.GetAggregationHealth)

	router.HandleFunc("POST /aggregation/trigger", handler.TriggerManualAggregation)
}
