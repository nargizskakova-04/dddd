// internal/adapters/handler/http/v1/endpoints.go
package v1

import "net/http"

// SetMarketRoutes sets up all market data API routes
func SetMarketRoutes(router *http.ServeMux, marketHandler *PriceHandler, healthHandler *HealthHandler, exchangeHandler *ExchangeHandler) {
	// Market Data API Routes
	setPriceRoutes(marketHandler, router)

	// Data Mode API Routes
	setModeRoutes(exchangeHandler, router)

	// System Health Routes
	setHealthRoutes(healthHandler, router)
}

// setPriceRoutes sets up all price-related endpoints
func setPriceRoutes(handler *PriceHandler, router *http.ServeMux) {
	// Latest Price Endpoints
	router.HandleFunc("GET /prices/latest/{symbol}", handler.GetLatestPrice)
	router.HandleFunc("GET /prices/latest/{exchange}/{symbol}", handler.GetLatestPriceByExchange)

	// Highest Price Endpoints
	router.HandleFunc("GET /prices/highest/{symbol}", handler.GetHighestPrice)                      // Default period
	router.HandleFunc("GET /prices/highest/{exchange}/{symbol}", handler.GetHighestPriceByExchange) // Default period
	// Note: Same endpoints handle ?period={duration} query parameter for custom periods

	// Lowest Price Endpoints
	router.HandleFunc("GET /prices/lowest/{symbol}", handler.GetLowestPrice)                      // Default period
	router.HandleFunc("GET /prices/lowest/{exchange}/{symbol}", handler.GetLowestPriceByExchange) // Default period
	// Note: Same endpoints handle ?period={duration} query parameter for custom periods

	// Average Price Endpoints
	router.HandleFunc("GET /prices/average/{symbol}", handler.GetAveragePrice)                      // Default period
	router.HandleFunc("GET /prices/average/{exchange}/{symbol}", handler.GetAveragePriceByExchange) // Default period
	// Note: Same endpoints handle ?period={duration} query parameter for custom periods
}

// setModeRoutes sets up data mode switching endpoints
func setModeRoutes(handler *ExchangeHandler, router *http.ServeMux) {
	// Mode switching endpoints
	router.HandleFunc("POST /mode/test", handler.SwitchToTestMode)
	router.HandleFunc("POST /mode/live", handler.SwitchToLiveMode)
	router.HandleFunc("POST /mode/all", handler.SwitchToAllMode) // NEW: All mode endpoint

	// Mode information endpoints
	router.HandleFunc("GET /mode/current", handler.GetCurrentMode) // Get current mode info
	router.HandleFunc("GET /mode/stats", handler.GetServiceStats)  // NEW: Get service statistics
}

// setHealthRoutes sets up system health endpoints
func setHealthRoutes(handler *HealthHandler, router *http.ServeMux) {
	router.HandleFunc("GET /health", handler.GetSystemHealth)
	router.HandleFunc("GET /health/detailed", handler.GetDetailedHealth) // Extra: detailed health check
}
