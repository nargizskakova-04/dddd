package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"cryptomarket/internal/adapters/cache"
	v1 "cryptomarket/internal/adapters/handler/http/v1"
	"cryptomarket/internal/adapters/repository/postgres"
	"cryptomarket/internal/config"
	"cryptomarket/internal/core/domain"
	"cryptomarket/internal/core/port"
	"cryptomarket/internal/core/service/exchange"
	"cryptomarket/internal/core/service/prices"

	"github.com/redis/go-redis/v9"

	_ "github.com/lib/pq"
)

type App struct {
	cfg          *config.Config
	router       *http.ServeMux
	db           *sql.DB
	redisClient  *redis.Client
	cacheAdapter port.Cache

	// Services
	exchangeService port.ExchangeService
	priceService    port.PriceService

	// For graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

	// Batch processing
	batchMutex sync.Mutex
	batch      []domain.MarketData
	batchSize  int
}

func NewApp(cfg *config.Config) *App {
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		batch:     make([]domain.MarketData, 0),
		batchSize: 100, // Process in batches of 100
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

	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Warn("Redis connection failed, continuing without cache", "error", err)
		app.redisClient = nil
		app.cacheAdapter = nil
	} else {
		app.redisClient = redisClient
		app.cacheAdapter = cache.NewRedisAdapter(redisClient)
		slog.Info("Redis connected successfully")
	}

	// Initialize services following hexagonal architecture

	// 1. Create Exchange Service (handles data collection) - now with cache
	app.exchangeService = exchange.NewExchangeService(app.cfg, app.cacheAdapter)

	// 2. Create Price Service (business logic layer)
	app.priceService = prices.NewPriceService(app.cacheAdapter, app.db)

	// 3. Create Handlers (adapters layer)
	priceHandler := v1.NewPriceHandler(app.priceService)
	healthHandler := v1.NewHealthHandler(app.createHealthService())
	modeHandler := v1.NewModeHandler(app.createModeService())

	// 4. Set up routes
	v1.SetMarketRoutes(app.router, priceHandler, healthHandler, modeHandler)

	// Add debug routes
	app.router.HandleFunc("GET /debug/cache/clear", app.handleCacheClear)
	app.router.HandleFunc("GET /debug/stats", app.handleDebugStats)

	// 5. Start background data processing
	go app.startMarketDataProcessor()

	slog.Info("Application initialized successfully")
	return nil
}

func (app *App) Run() {
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", app.cfg.App.Port),
		Handler: app.router,
	}

	slog.Info("Starting server", "port", app.cfg.App.Port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("Server error", "error", err)
		return
	}
}

// Debug handlers
func (app *App) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if app.cacheAdapter == nil {
		http.Error(w, "Cache not available", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := app.cacheAdapter.CleanupOldData(ctx, 0); err != nil {
		http.Error(w, "Failed to clear cache: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message": "Cache cleared successfully"}`))
}

func (app *App) handleDebugStats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]interface{})

	// Exchange service stats
	if app.exchangeService != nil {
		stats["exchange_service"] = app.exchangeService.GetStats()
	}

	// Current mode
	if app.exchangeService != nil {
		stats["current_mode"] = app.exchangeService.GetCurrentMode()
	}

	// Cache status
	if app.redisClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := app.redisClient.Ping(ctx).Err(); err != nil {
			stats["cache_status"] = "unhealthy: " + err.Error()
		} else {
			stats["cache_status"] = "healthy"
		}
	} else {
		stats["cache_status"] = "unavailable"
	}

	// Batch processing stats
	app.batchMutex.Lock()
	batchSize := len(app.batch)
	app.batchMutex.Unlock()

	stats["batch_processing"] = map[string]interface{}{
		"current_batch_size": batchSize,
		"max_batch_size":     app.batchSize,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, "Failed to encode stats", http.StatusInternalServerError)
	}
}

// Background task for processing market data
func (app *App) startMarketDataProcessor() {
	slog.Info("Starting market data processor...")

	// Start exchange service in live mode by default
	if err := app.exchangeService.SwitchToLiveMode(app.ctx); err != nil {
		slog.Error("Failed to switch to live mode", "error", err)
		return
	}

	// Start data processing
	if err := app.exchangeService.StartDataProcessing(app.ctx); err != nil {
		slog.Error("Failed to start data processing", "error", err)
		return
	}

	// Get data stream from exchange service
	dataStream := app.exchangeService.GetDataStream()

	// Start multiple processors for better performance
	numProcessors := 3
	for i := 0; i < numProcessors; i++ {
		go app.processMarketData(dataStream, i)
	}

	// Start batch flusher
	go app.batchFlusher()

	// Start cleanup routine for Redis
	if app.cacheAdapter != nil {
		go app.startCleanupRoutine()
	}

	slog.Info("Market data processor started successfully", "processors", numProcessors)
}

// processMarketData handles incoming market data and stores it in cache (optimized)
func (app *App) processMarketData(dataStream <-chan domain.MarketData, processorID int) {
	slog.Info("Starting market data processing goroutine", "processor", processorID)
	processedCount := 0
	lastLogTime := time.Now()

	for {
		select {
		case data, ok := <-dataStream:
			if !ok {
				slog.Info("Market data stream closed", "processor", processorID, "processed", processedCount)
				return
			}

			processedCount++

			// Log stats every 30 seconds instead of every 100 messages
			now := time.Now()
			if now.Sub(lastLogTime) > 30*time.Second {
				slog.Info("Processing market data stats",
					"processor", processorID,
					"count", processedCount,
					"rate", float64(processedCount)/now.Sub(lastLogTime.Add(-30*time.Second)).Seconds())
				lastLogTime = now
			}

			// Add to batch for Redis processing
			app.addToBatch(data)

		case <-app.ctx.Done():
			slog.Info("Market data processing stopped", "processor", processorID, "processed", processedCount)
			return
		}
	}
}

// addToBatch adds data to batch for efficient processing
func (app *App) addToBatch(data domain.MarketData) {
	app.batchMutex.Lock()
	defer app.batchMutex.Unlock()

	app.batch = append(app.batch, data)

	// If batch is full, process immediately
	if len(app.batch) >= app.batchSize {
		app.processBatchUnsafe()
	}
}

// batchFlusher periodically flushes partial batches
func (app *App) batchFlusher() {
	ticker := time.NewTicker(2 * time.Second) // Flush every 2 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			app.batchMutex.Lock()
			if len(app.batch) > 0 {
				app.processBatchUnsafe()
			}
			app.batchMutex.Unlock()

		case <-app.ctx.Done():
			slog.Info("Batch flusher stopped")
			// Process remaining batch
			app.batchMutex.Lock()
			if len(app.batch) > 0 {
				app.processBatchUnsafe()
			}
			app.batchMutex.Unlock()
			return
		}
	}
}

// processBatchUnsafe processes current batch (must be called with batchMutex held)
func (app *App) processBatchUnsafe() {
	if len(app.batch) == 0 {
		return
	}

	batchSize := len(app.batch)
	slog.Debug("Processing batch", "size", batchSize)

	// Process batch asynchronously to avoid blocking
	batchCopy := make([]domain.MarketData, len(app.batch))
	copy(batchCopy, app.batch)
	app.batch = app.batch[:0] // Clear batch

	// Process in background
	go app.processBatchAsync(batchCopy)
}

// processBatchAsync processes a batch of market data asynchronously
func (app *App) processBatchAsync(batch []domain.MarketData) {
	if app.cacheAdapter == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	successCount := 0
	errorCount := 0

	// Process in smaller sub-batches for better performance
	subBatchSize := 20
	for i := 0; i < len(batch); i += subBatchSize {
		end := i + subBatchSize
		if end > len(batch) {
			end = len(batch)
		}

		// Process sub-batch
		for j := i; j < end; j++ {
			data := batch[j]
			key := fmt.Sprintf("%s:%s", data.Symbol, data.Exchange)

			if err := app.cacheAdapter.SetPrice(ctx, key, data); err != nil {
				errorCount++
				if errorCount%10 == 1 { // Log every 10th error to avoid spam
					slog.Error("Failed to store price in cache",
						"error", err,
						"symbol", data.Symbol,
						"exchange", data.Exchange)
				}
			} else {
				successCount++
			}
		}

		// Small delay between sub-batches to avoid overwhelming Redis
		select {
		case <-time.After(1 * time.Millisecond):
		case <-ctx.Done():
			return
		}
	}

	if errorCount > 0 {
		slog.Warn("Batch processing completed with errors",
			"total", len(batch),
			"success", successCount,
			"errors", errorCount)
	} else {
		slog.Debug("Batch processing completed successfully",
			"processed", successCount)
	}
}

// startCleanupRoutine cleans up old data from Redis (optimized)
func (app *App) startCleanupRoutine() {
	ticker := time.NewTicker(60 * time.Second) // Clean up every minute
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			// Clean up data older than 5 minutes (increased from 2)
			if err := app.cacheAdapter.CleanupOldData(ctx, 5*time.Minute); err != nil {
				slog.Error("Failed to cleanup old data", "error", err)
			} else {
				slog.Debug("Cleanup completed successfully")
			}

			cancel()

		case <-app.ctx.Done():
			slog.Info("Cleanup routine stopped")
			return
		}
	}
}

// createModeService creates a simple mode service
func (app *App) createModeService() port.ModeService {
	return &SimpleModeService{
		exchangeService: app.exchangeService,
	}
}

// createHealthService creates a simple health service
func (app *App) createHealthService() port.HealthService {
	return &SimpleHealthService{
		exchangeService: app.exchangeService,
		redisClient:     app.redisClient,
		db:              app.db,
	}
}

// Shutdown gracefully shuts down the application
func (app *App) Shutdown() error {
	slog.Info("Shutting down application...")

	// Cancel context to stop all goroutines
	app.cancel()

	// Process remaining batch
	app.batchMutex.Lock()
	if len(app.batch) > 0 {
		slog.Info("Processing remaining batch before shutdown", "size", len(app.batch))
		app.processBatchUnsafe()
	}
	app.batchMutex.Unlock()

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

// SimpleModeService implements basic mode switching
type SimpleModeService struct {
	exchangeService port.ExchangeService
}

func (s *SimpleModeService) SwitchToTestMode(ctx context.Context) error {
	return s.exchangeService.SwitchToTestMode(ctx)
}

func (s *SimpleModeService) SwitchToLiveMode(ctx context.Context) error {
	return s.exchangeService.SwitchToLiveMode(ctx)
}

func (s *SimpleModeService) GetCurrentMode() string {
	return s.exchangeService.GetCurrentMode()
}

// SimpleHealthService implements basic health checking
type SimpleHealthService struct {
	exchangeService port.ExchangeService
	redisClient     *redis.Client
	db              *sql.DB
}

func (s *SimpleHealthService) GetSystemHealth(ctx context.Context) map[string]interface{} {
	health := make(map[string]interface{})

	// Exchange service health
	if s.exchangeService != nil {
		stats := s.exchangeService.GetStats()
		health["exchange_service"] = stats
	} else {
		health["exchange_service"] = "unavailable"
	}

	// Redis health
	if s.redisClient != nil {
		if err := s.redisClient.Ping(ctx).Err(); err != nil {
			health["redis"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
		} else {
			health["redis"] = map[string]interface{}{
				"status": "healthy",
			}
		}
	} else {
		health["redis"] = "unavailable"
	}

	// Database health
	if s.db != nil {
		if err := s.db.PingContext(ctx); err != nil {
			health["database"] = map[string]interface{}{
				"status": "unhealthy",
				"error":  err.Error(),
			}
		} else {
			health["database"] = map[string]interface{}{
				"status": "healthy",
			}
		}
	} else {
		health["database"] = "unavailable"
	}

	return health
}
