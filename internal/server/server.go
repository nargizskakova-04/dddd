package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"crypto/internal/adapters/cache"
	v1 "crypto/internal/adapters/handler/http/v1"
	"crypto/internal/adapters/repository/postgres"
	"crypto/internal/config"
	"crypto/internal/core/domain"
	"crypto/internal/core/port"
	"crypto/internal/core/service/exchange"
	"crypto/internal/core/service/prices"

	"github.com/redis/go-redis/v9"

	_ "github.com/lib/pq"
)

type App struct {
	cfg         *config.Config
	router      *http.ServeMux
	db          *sql.DB
	redisClient *redis.Client

	// Services
	exchangeService port.ExchangeService
	priceService    port.PriceService

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

	// 1. Create Exchange Service (handles data collection)
	app.exchangeService = exchange.NewExchangeService()

	// 2. Create Price Service (business logic layer)
	app.priceService = prices.NewPriceService(cacheAdapter, app.db)

	// 3. Create Handlers (adapters layer)
	priceHandler := v1.NewPriceHandler(app.priceService)
	healthHandler := v1.NewHealthHandler(nil) // TODO: implement health service
	modeHandler := v1.NewModeHandler(nil)     // TODO: implement mode service

	// 4. Set up routes
	v1.SetMarketRoutes(app.router, priceHandler, healthHandler, modeHandler)

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
		panic(err)
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

	// Process incoming market data
	go app.processMarketData(dataStream)

	// Start cleanup routine for Redis
	if app.redisClient != nil {
		go app.startCleanupRoutine()
	}

	slog.Info("Market data processor started successfully")
}

// processMarketData handles incoming market data and stores it in cache
func (app *App) processMarketData(dataStream <-chan domain.MarketData) {
	slog.Info("Starting market data processing goroutine...")

	for {
		select {
		case data, ok := <-dataStream:
			if !ok {
				slog.Info("Market data stream closed")
				return
			}

			// Store in Redis cache if available
			if app.redisClient != nil {
				cacheAdapter := cache.NewRedisAdapter(app.redisClient).(*cache.RedisAdapter)
				key := fmt.Sprintf("%s:%s", data.Symbol, data.Exchange)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := cacheAdapter.SetPrice(ctx, key, data); err != nil {
					slog.Error("Failed to store price in cache", "error", err, "symbol", data.Symbol, "exchange", data.Exchange)
				}
				cancel()
			}

			// TODO: Implement batching and PostgreSQL storage
			// This should batch data and store aggregated statistics every minute

		case <-app.ctx.Done():
			slog.Info("Market data processing stopped")
			return
		}
	}
}

// startCleanupRoutine cleans up old data from Redis
func (app *App) startCleanupRoutine() {
	ticker := time.NewTicker(30 * time.Second) // Clean up every 30 seconds
	defer ticker.Stop()

	cacheAdapter := cache.NewRedisAdapter(app.redisClient).(*cache.RedisAdapter)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

			// Clean up data older than 2 minutes
			if err := cacheAdapter.CleanupOldData(ctx, 2*time.Minute); err != nil {
				slog.Error("Failed to cleanup old data", "error", err)
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

	// Cancel context to stop all goroutines
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
