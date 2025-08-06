package health

import (
	"context"
	"time"

	"cryptomarket/internal/core/port"
	"cryptomarket/internal/core/service/aggregation"
)

type HealthService struct {
	healthRepo         port.HealthRepository
	cache              port.Cache
	exchangeService    port.ExchangeService
	aggregationService *aggregation.AggregationService
}

func NewHealthService(
	healthRepo port.HealthRepository,
	cache port.Cache,
	exchangeService port.ExchangeService,
	aggregationService *aggregation.AggregationService,
) port.HealthService {
	return &HealthService{
		healthRepo:         healthRepo,
		cache:              cache,
		exchangeService:    exchangeService,
		aggregationService: aggregationService,
	}
}

func (h *HealthService) GetSystemHealth(ctx context.Context) map[string]interface{} {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	// Check critical components
	dbHealthy := h.checkDatabaseHealth(ctx)
	cacheHealthy := h.checkCacheHealth(ctx)
	exchangeHealthy := h.checkExchangeHealth(ctx)

	// Determine overall status
	if !dbHealthy || !cacheHealthy || !exchangeHealthy {
		health["status"] = "unhealthy"
	}

	// Add component statuses
	health["database"] = map[string]interface{}{
		"status": getStatusString(dbHealthy),
	}

	health["cache"] = map[string]interface{}{
		"status": getStatusString(cacheHealthy),
	}

	health["exchange_service"] = map[string]interface{}{
		"status": getStatusString(exchangeHealthy),
	}

	if h.aggregationService != nil {
		aggHealth := h.aggregationService.GetHealthStatus(ctx)
		health["aggregation_service"] = map[string]interface{}{
			"status":  "available",
			"details": aggHealth,
		}
	} else {
		health["aggregation_service"] = map[string]interface{}{
			"status": "unavailable",
		}
	}

	return health
}

func (h *HealthService) GetDetailedHealth(ctx context.Context) map[string]interface{} {
	health := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	// Database health
	dbHealth := h.getDetailedDatabaseHealth(ctx)
	health["database"] = dbHealth

	// Cache health
	cacheHealth := h.getDetailedCacheHealth(ctx)
	health["cache"] = cacheHealth

	// Exchange service health
	exchangeHealth := h.getDetailedExchangeHealth(ctx)
	health["exchange_service"] = exchangeHealth

	// Aggregation service health
	if h.aggregationService != nil {
		aggHealth := h.aggregationService.GetHealthStatus(ctx)
		health["aggregation_service"] = map[string]interface{}{
			"status":  "available",
			"details": aggHealth,
		}
	} else {
		health["aggregation_service"] = map[string]interface{}{
			"status": "unavailable",
			"reason": "service not initialized",
		}
	}

	// Overall system status
	allHealthy := h.IsHealthy(ctx)
	health["overall_status"] = getStatusString(allHealthy)

	return health
}

func (h *HealthService) IsHealthy(ctx context.Context) bool {
	return h.checkDatabaseHealth(ctx) &&
		h.checkCacheHealth(ctx) &&
		h.checkExchangeHealth(ctx)
}

// Private helper methods

func (h *HealthService) checkDatabaseHealth(ctx context.Context) bool {
	if h.healthRepo == nil {
		return false
	}

	return h.healthRepo.CheckDatabaseHealth(ctx) == nil
}

func (h *HealthService) checkCacheHealth(ctx context.Context) bool {
	if h.cache == nil {
		return false // Cache is considered critical
	}

	return h.cache.Ping(ctx) == nil
}

func (h *HealthService) checkExchangeHealth(ctx context.Context) bool {
	if h.exchangeService == nil {
		return false
	}

	return h.exchangeService.IsRunning()
}

func (h *HealthService) getDetailedDatabaseHealth(ctx context.Context) map[string]interface{} {
	dbHealth := map[string]interface{}{}

	if h.healthRepo == nil {
		dbHealth["status"] = "unavailable"
		dbHealth["reason"] = "repository not initialized"
		return dbHealth
	}

	if err := h.healthRepo.CheckDatabaseHealth(ctx); err != nil {
		dbHealth["status"] = "unhealthy"
		dbHealth["error"] = err.Error()
	} else {
		dbHealth["status"] = "healthy"
		dbHealth["connection"] = "active"
	}

	return dbHealth
}

func (h *HealthService) getDetailedCacheHealth(ctx context.Context) map[string]interface{} {
	cacheHealth := map[string]interface{}{}

	if h.cache == nil {
		cacheHealth["status"] = "unavailable"
		cacheHealth["reason"] = "cache not initialized"
		return cacheHealth
	}

	if err := h.cache.Ping(ctx); err != nil {
		cacheHealth["status"] = "unhealthy"
		cacheHealth["error"] = err.Error()
	} else {
		cacheHealth["status"] = "healthy"
		cacheHealth["connection"] = "active"
	}

	return cacheHealth
}

func (h *HealthService) getDetailedExchangeHealth(ctx context.Context) map[string]interface{} {
	exchangeHealth := map[string]interface{}{}

	if h.exchangeService == nil {
		exchangeHealth["status"] = "unavailable"
		exchangeHealth["reason"] = "service not initialized"
		return exchangeHealth
	}

	exchangeHealth["status"] = getStatusString(h.exchangeService.IsRunning())
	exchangeHealth["current_mode"] = h.exchangeService.GetCurrentMode()

	// Get adapter health
	if svc, ok := h.exchangeService.(interface{ GetStats() map[string]interface{} }); ok {
		stats := svc.GetStats()
		exchangeHealth["stats"] = stats
	}

	// Get adapter details
	if svc, ok := h.exchangeService.(interface{ GetAllAdapters() []port.ExchangeAdapter }); ok {
		adapters := svc.GetAllAdapters()
		adapterStatus := make(map[string]interface{})

		healthyCount := 0
		for _, adapter := range adapters {
			isHealthy := adapter.IsHealthy()
			adapterStatus[adapter.Name()] = map[string]interface{}{
				"healthy": isHealthy,
				"status":  getStatusString(isHealthy),
			}
			if isHealthy {
				healthyCount++
			}
		}

		exchangeHealth["adapters"] = adapterStatus
		exchangeHealth["total_adapters"] = len(adapters)
		exchangeHealth["healthy_adapters"] = healthyCount
	}

	return exchangeHealth
}

func getStatusString(healthy bool) string {
	if healthy {
		return "healthy"
	}
	return "unhealthy"
}
