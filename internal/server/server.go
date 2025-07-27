// internal/server/server.go - Updated with aggregation service
package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"cryptomarket/internal/adapters/cache"
	v1 "cryptomarket/internal/adapters/handler/http/v1"
	"cryptomarket/internal/adapters/repository/postgres"
	"cryptomarket/internal/config"
	"cryptomarket/internal/core/domain"
	"cryptomarket/internal/core/port"
	"cryptomarket/internal/core/service/aggregation"
	"cryptomarket/internal/core/service/exchange"
	"cryptomarket/internal/core/service/prices"

	"github.com/redis/go-redis/v9"

	_ "github.com/lib/pq"
)

type App struct {
	cfg         *config.Config
	router      *http.ServeMux
	db          *sql.DB
	redisClient *redis.Client

	// Services
	exchangeService    port.ExchangeService
	priceService       port.PriceService
	aggregationService *aggregation.AggregationService

	// For graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

func NewApp(cfg *config.Config) *App {
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (app *App) Initialize() error {
	slog.Info("Initializing application...")
	app.router = http.NewServeMux()

	// Database connection
	dbConn, err := postgres.NewDbConnInstance(&app.cfg.Repository)
	if err != nil {
		slog.Error("Connection to database failed", "error", err)
		return err
	}
	app.db = dbConn
	slog.Info("Database connected successfully")

	// Redis connection
	redisClient := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", app.cfg.Cache.RedisHost, app.cfg.Cache.RedisPort),
		Password:     app.cfg.Cache.RedisPassword,
		DB:           app.cfg.Cache.RedisDB,
		PoolSize:     app.cfg.Cache.PoolSize,
		MinIdleConns: app.cfg.Cache.MinIdleConns,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cacheAdapter *cache.RedisAdapter
	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Warn("Redis connection failed, continuing without cache", "error", err)
		app.redisClient = nil
		cacheAdapter = nil
	} else {
		app.redisClient = redisClient
		cacheAdapter = cache.NewRedisAdapter(redisClient).(*cache.RedisAdapter)
		slog.Info("Redis connected successfully")
	}

	// Initialize services following hexagonal architecture

	// 1. Create Exchange Service (handles data collection from ALL exchanges)
	app.exchangeService = exchange.NewExchangeService()
	slog.Info("Exchange service created", "default_mode", app.exchangeService.GetCurrentMode())

	// 2. Create Price Service (business logic layer) - now with exchange service dependency
	app.priceService = prices.NewPriceService(cacheAdapter, app.db, app.exchangeService)
	slog.Info("Price service created with mode awareness")

	// 3. NEW: Create Aggregation Service for Redis -> PostgreSQL data aggregation
	if cacheAdapter != nil && app.db != nil {
		pricesRepo := postgres.NewPricesRepository(app.db)
		app.aggregationService = aggregation.NewAggregationService(cacheAdapter, pricesRepo)
		slog.Info("Aggregation service created successfully")
	} else {
		slog.Warn("Aggregation service not created - missing cache or database")
	}

	// 4. Create Handlers (adapters layer)
	priceHandler := v1.NewPriceHandler(app.priceService)
	healthHandler := v1.NewHealthHandler(nil) // TODO: implement health service
	modeHandler := v1.NewExchangeHandler(app.exchangeService)

	// NEW: Create aggregation handler
	var aggregationHandler *v1.AggregationHandler
	if app.aggregationService != nil {
		aggregationHandler = v1.NewAggregationHandler(app.aggregationService)
	}

	// 5. Set up routes
	v1.SetMarketRoutes(app.router, priceHandler, healthHandler, modeHandler, aggregationHandler)
	slog.Info("HTTP routes configured")

	// 6. Start background data processing for ALL exchanges
	go app.startMarketDataProcessor()

	// 7. NEW: Start background aggregation service
	if app.aggregationService != nil {
		go app.startAggregationService()
	}

	slog.Info("Application initialized successfully")
	slog.Info("System will collect data from ALL 6 exchanges but serve data based on current mode",
		"current_mode", app.exchangeService.GetCurrentMode(),
		"allowed_exchanges", app.exchangeService.GetModeExchanges())
	return nil
}

func (app *App) Run() {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", app.cfg.App.Port),
		Handler: app.router,
	}

	slog.Info("Starting server", "port", app.cfg.App.Port)
	slog.Info("Available endpoints:",
		"mode_switch", "POST /mode/{live|test|all}",
		"mode_info", "GET /mode/current",
		"service_stats", "GET /mode/stats",
		"price_latest", "GET /prices/latest/{symbol}",
		"price_by_exchange", "GET /prices/latest/{exchange}/{symbol}",
		"aggregation_status", "GET /aggregation/status",
		"aggregation_health", "GET /aggregation/health",
		"trigger_aggregation", "POST /aggregation/trigger")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server error", "error", err)
		return
	}
}

// Background task for processing market data from ALL exchanges
func (app *App) startMarketDataProcessor() {
	slog.Info("Starting market data processor for ALL exchanges...")

	// Set to live mode by default (only affects data reading, not collection)
	if err := app.exchangeService.SwitchToLiveMode(app.ctx); err != nil {
		slog.Error("Failed to set default live mode", "error", err)
	}

	// Start data processing for ALL exchanges (both live and test)
	if err := app.exchangeService.StartDataProcessing(app.ctx); err != nil {
		slog.Error("Failed to start data processing", "error", err)
		return
	}

	// Get data stream from exchange service (contains data from ALL exchanges)
	dataStream := app.exchangeService.GetDataStream()

	// Process incoming market data and store ALL data in Redis
	go app.processMarketData(dataStream)

	// Start cleanup routine for Redis
	if app.redisClient != nil {
		go app.startCleanupRoutine()
	}

	slog.Info("Market data processor started successfully")
	slog.Info("System is now collecting and storing data from all 6 exchanges")
}

// NEW: Start the aggregation service for Redis -> PostgreSQL data aggregation
func (app *App) startAggregationService() {
	slog.Info("Starting Redis -> PostgreSQL aggregation service...")

	if app.aggregationService == nil {
		slog.Error("Aggregation service is not available")
		return
	}

	// Check health status before starting
	healthStatus := app.aggregationService.GetHealthStatus(app.ctx)
	slog.Info("Aggregation service health check", "status", healthStatus)

	// Start periodic aggregation (runs every minute)
	app.aggregationService.PerformPeriodicAggregation(app.ctx)
}

// processMarketData handles incoming market data from ALL exchanges and stores it in cache
func (app *App) processMarketData(dataStream <-chan domain.MarketData) {
	slog.Info("Starting market data processing goroutine for ALL exchanges...")

	processedCount := make(map[string]int) // Track processed count per exchange

	for {
		select {
		case data, ok := <-dataStream:
			if !ok {
				slog.Info("Market data stream closed")
				return
			}

			// Store in Redis cache if available (store ALL data regardless of current mode)
			if app.redisClient != nil {
				cacheAdapter := cache.NewRedisAdapter(app.redisClient).(*cache.RedisAdapter)
				key := fmt.Sprintf("%s:%s", data.Symbol, data.Exchange)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := cacheAdapter.SetPrice(ctx, key, data); err != nil {
					slog.Error("Failed to store price in cache", "error", err, "symbol", data.Symbol, "exchange", data.Exchange)
				} else {
					processedCount[data.Exchange]++
					if processedCount[data.Exchange]%100 == 0 { // Log every 100 processed items per exchange
						slog.Debug("Processed market data", "exchange", data.Exchange, "count", processedCount[data.Exchange])
					}
				}
				cancel()
			}

		case <-app.ctx.Done():
			slog.Info("Market data processing stopped")
			totalProcessed := 0
			for exchange, count := range processedCount {
				totalProcessed += count
				slog.Info("Final processing count", "exchange", exchange, "processed", count)
			}
			slog.Info("Total market data processed", "count", totalProcessed)
			return
		}
	}
}

// startCleanupRoutine cleans up old data from Redis
func (app *App) startCleanupRoutine() {
	ticker := time.NewTicker(60 * time.Second) // Clean up every minute
	defer ticker.Stop()

	cacheAdapter := cache.NewRedisAdapter(app.redisClient).(*cache.RedisAdapter)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			// Clean up data older than 2 minutes
			if err := cacheAdapter.CleanupOldData(ctx, 2*time.Minute); err != nil {
				slog.Error("Failed to cleanup old data", "error", err)
			} else {
				slog.Debug("Cleaned up old data from Redis")
			}

			cancel()

		case <-app.ctx.Done():
			slog.Info("Cleanup routine stopped")
			return
		}
	}
}

// Shutdown gracefully shuts down the application
func (app *App) Shutdown() error {
	slog.Info("Shutting down application...")

	// Cancel context to stop all goroutines (including aggregation service)
	app.cancel()

	// Stop exchange service
	if app.exchangeService != nil {
		if err := app.exchangeService.StopDataProcessing(); err != nil {
			slog.Error("Failed to stop exchange service", "error", err)
		}
	}

	// Close database connection
	if app.db != nil {
		if err := app.db.Close(); err != nil {
			slog.Error("Failed to close database", "error", err)
		}
	}

	// Close Redis connection
	if app.redisClient != nil {
		if err := app.redisClient.Close(); err != nil {
			slog.Error("Failed to close Redis", "error", err)
		}
	}

	slog.Info("Application shutdown complete")
	return nil
}
