package exchange

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"cryptomarket/internal/adapters/exchanges"
	"cryptomarket/internal/config"
	"cryptomarket/internal/core/domain"
	"cryptomarket/internal/core/port"
)

// ExchangeService implements the port.ExchangeService interface
type ExchangeService struct {
	// Configuration
	config *config.Config

	// Mode management
	currentMode string
	modeMutex   sync.RWMutex

	// Exchange adapters
	liveAdapters   []port.ExchangeAdapter
	testAdapters   []port.ExchangeAdapter
	activeAdapters []port.ExchangeAdapter

	// Concurrency channels
	aggregatedDataChan chan domain.MarketData
	workerPool         []chan domain.MarketData
	resultChan         chan domain.MarketData

	// Control
	ctx       context.Context
	cancel    context.CancelFunc
	isRunning bool
	runMutex  sync.RWMutex
	wg        sync.WaitGroup

	// Configuration
	numWorkers int

	// Cache interface for cleanup
	cache port.Cache
}

// NewExchangeService creates a new exchange service with configuration
func NewExchangeService(cfg *config.Config, cache port.Cache) port.ExchangeService {
	ctx, cancel := context.WithCancel(context.Background())

	// Create live adapters from configuration
	liveAdapters := make([]port.ExchangeAdapter, 0)
	for _, exchangeConfig := range cfg.Exchanges.LiveExchanges {
		adapter := exchanges.NewLiveExchangeAdapter(
			exchangeConfig.Host,
			exchangeConfig.Port,
			exchangeConfig.Name,
		)
		liveAdapters = append(liveAdapters, adapter)
		slog.Info("Created live adapter", "name", exchangeConfig.Name, "port", exchangeConfig.Port)
	}

	// Create test adapters from configuration
	testAdapters := make([]port.ExchangeAdapter, 0)
	for _, exchangeConfig := range cfg.Exchanges.TestExchanges {
		adapter := exchanges.NewLiveExchangeAdapter(
			exchangeConfig.Host,
			exchangeConfig.Port,
			exchangeConfig.Name,
		)
		testAdapters = append(testAdapters, adapter)
		slog.Info("Created test adapter", "name", exchangeConfig.Name, "port", exchangeConfig.Port)
	}

	numWorkers := 15 // 5 workers per exchange

	return &ExchangeService{
		config:             cfg,
		currentMode:        "live", // Default to live mode
		liveAdapters:       liveAdapters,
		testAdapters:       testAdapters,
		activeAdapters:     liveAdapters,
		aggregatedDataChan: make(chan domain.MarketData, 1000),
		workerPool:         make([]chan domain.MarketData, numWorkers),
		resultChan:         make(chan domain.MarketData, 1000),
		ctx:                ctx,
		cancel:             cancel,
		numWorkers:         numWorkers,
		cache:              cache,
	}
}

func (e *ExchangeService) SwitchToLiveMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == "live" {
		slog.Info("Already in live mode, no action needed")
		return nil
	}

	slog.Info("Switching to live mode...")

	// Stop data processing
	e.runMutex.Lock()
	wasRunning := e.isRunning
	e.runMutex.Unlock()

	if wasRunning {
		slog.Info("Stopping data processing before mode switch...")
		if err := e.stopDataProcessingUnsafe(); err != nil {
			return fmt.Errorf("failed to stop data processing: %w", err)
		}
	}

	// Switch to live adapters
	e.activeAdapters = e.liveAdapters
	e.currentMode = "live"

	// Clear cache to remove old test data
	if err := e.clearCacheData(ctx); err != nil {
		slog.Warn("Failed to clear cache data", "error", err)
	}

	// Restart data processing if it was running
	if wasRunning {
		slog.Info("Restarting data processing in live mode...")
		if err := e.startDataProcessingUnsafe(ctx); err != nil {
			return fmt.Errorf("failed to restart data processing: %w", err)
		}
	}

	slog.Info("Successfully switched to live mode")
	return nil
}

func (e *ExchangeService) SwitchToTestMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == "test" {
		slog.Info("Already in test mode, no action needed")
		return nil
	}

	slog.Info("Switching to test mode...")

	// Stop data processing
	e.runMutex.Lock()
	wasRunning := e.isRunning
	e.runMutex.Unlock()

	if wasRunning {
		slog.Info("Stopping data processing before mode switch...")
		if err := e.stopDataProcessingUnsafe(); err != nil {
			return fmt.Errorf("failed to stop data processing: %w", err)
		}
	}

	// Switch to test adapters
	e.activeAdapters = e.testAdapters
	e.currentMode = "test"

	// Clear cache to remove old live data
	if err := e.clearCacheData(ctx); err != nil {
		slog.Warn("Failed to clear cache data", "error", err)
	}

	// Restart data processing if it was running
	if wasRunning {
		slog.Info("Restarting data processing in test mode...")
		if err := e.startDataProcessingUnsafe(ctx); err != nil {
			return fmt.Errorf("failed to restart data processing: %w", err)
		}
	}

	slog.Info("Successfully switched to test mode")
	return nil
}

func (e *ExchangeService) GetCurrentMode() string {
	e.modeMutex.RLock()
	defer e.modeMutex.RUnlock()
	return e.currentMode
}

func (e *ExchangeService) StartDataProcessing(ctx context.Context) error {
	e.runMutex.Lock()
	defer e.runMutex.Unlock()
	return e.startDataProcessingUnsafe(ctx)
}

func (e *ExchangeService) startDataProcessingUnsafe(ctx context.Context) error {
	if e.isRunning {
		return nil // Already running
	}

	e.modeMutex.RLock()
	currentMode := e.currentMode
	adapters := make([]port.ExchangeAdapter, len(e.activeAdapters))
	copy(adapters, e.activeAdapters)
	e.modeMutex.RUnlock()

	slog.Info("Starting data processing...",
		"mode", currentMode,
		"workers", e.numWorkers,
		"adapters", len(adapters))

	// Initialize worker pool channels
	for i := 0; i < e.numWorkers; i++ {
		e.workerPool[i] = make(chan domain.MarketData, 100)
	}

	// Start exchange adapters and collect their channels
	var inputChannels []<-chan domain.MarketData
	for _, adapter := range adapters {
		slog.Info("Starting adapter...", "name", adapter.Name())
		dataChan, err := adapter.Start(e.ctx)
		if err != nil {
			slog.Error("Failed to start adapter", "adapter", adapter.Name(), "error", err)
			continue
		}
		inputChannels = append(inputChannels, dataChan)
		slog.Info("Successfully started adapter", "name", adapter.Name(), "healthy", adapter.IsHealthy())
	}

	if len(inputChannels) == 0 {
		return fmt.Errorf("no exchange adapters started successfully")
	}

	// Start concurrency pipeline
	e.wg.Add(1)
	go e.fanIn(inputChannels)

	e.wg.Add(1)
	go e.distributor()

	for i := 0; i < e.numWorkers; i++ {
		e.wg.Add(1)
		go e.worker(i, e.workerPool[i])
	}

	e.isRunning = true
	slog.Info("Data processing started successfully",
		"mode", currentMode,
		"exchanges", len(inputChannels),
		"workers", e.numWorkers)
	return nil
}

func (e *ExchangeService) StopDataProcessing() error {
	e.runMutex.Lock()
	defer e.runMutex.Unlock()
	return e.stopDataProcessingUnsafe()
}

func (e *ExchangeService) stopDataProcessingUnsafe() error {
	if !e.isRunning {
		return nil // Already stopped
	}

	slog.Info("Stopping data processing...")

	// Stop all adapters
	if err := e.stopCurrentAdapters(); err != nil {
		slog.Error("Failed to stop adapters", "error", err)
	}

	// Cancel context to stop all goroutines
	e.cancel()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("All goroutines stopped")
	case <-time.After(10 * time.Second):
		slog.Warn("Timeout waiting for goroutines to stop")
	}

	// Close worker channels
	for i, workerChan := range e.workerPool {
		if workerChan != nil {
			close(workerChan)
			e.workerPool[i] = nil
		}
	}

	// Close result channel
	if e.resultChan != nil {
		close(e.resultChan)
	}

	// Recreate context and channels for next start
	e.ctx, e.cancel = context.WithCancel(context.Background())
	e.aggregatedDataChan = make(chan domain.MarketData, 1000)
	e.resultChan = make(chan domain.MarketData, 1000)
	e.workerPool = make([]chan domain.MarketData, e.numWorkers)

	e.isRunning = false
	slog.Info("Data processing stopped")
	return nil
}

func (e *ExchangeService) GetDataStream() <-chan domain.MarketData {
	return e.resultChan
}

// clearCacheData clears cache when switching modes
func (e *ExchangeService) clearCacheData(ctx context.Context) error {
	if e.cache == nil {
		return nil
	}

	slog.Info("Clearing cache data due to mode switch...")

	// Clear data older than 0 seconds (all data)
	if err := e.cache.CleanupOldData(ctx, 0); err != nil {
		return fmt.Errorf("failed to cleanup cache: %w", err)
	}

	slog.Info("Cache cleared successfully")
	return nil
}

// Fan-in: Aggregates data from multiple exchange channels into one
func (e *ExchangeService) fanIn(inputChannels []<-chan domain.MarketData) {
	defer e.wg.Done()
	defer close(e.aggregatedDataChan)

	slog.Info("Starting fan-in aggregator", "inputs", len(inputChannels))

	var fanInWg sync.WaitGroup

	// Start a goroutine for each input channel
	for i, ch := range inputChannels {
		fanInWg.Add(1)
		go func(id int, inputChan <-chan domain.MarketData) {
			defer fanInWg.Done()

			for {
				select {
				case data, ok := <-inputChan:
					if !ok {
						slog.Info("Input channel closed", "channel", id)
						return
					}

					slog.Debug("Received data from channel",
						"channel", id,
						"symbol", data.Symbol,
						"exchange", data.Exchange,
						"price", data.Price)

					select {
					case e.aggregatedDataChan <- data:
					case <-e.ctx.Done():
						return
					}

				case <-e.ctx.Done():
					return
				}
			}
		}(i, ch)
	}

	fanInWg.Wait()
	slog.Info("Fan-in aggregator completed")
}

// Distributor: Fan-out data to worker pool
func (e *ExchangeService) distributor() {
	defer e.wg.Done()

	slog.Info("Starting distributor", "workers", e.numWorkers)
	workerIndex := 0
	distributedCount := 0

	for {
		select {
		case data, ok := <-e.aggregatedDataChan:
			if !ok {
				slog.Info("Aggregated data channel closed", "distributed", distributedCount)
				return
			}

			// Round-robin distribution to workers
			select {
			case e.workerPool[workerIndex] <- data:
				distributedCount++
				if distributedCount%100 == 0 {
					slog.Debug("Distributor stats", "distributed", distributedCount)
				}
				workerIndex = (workerIndex + 1) % e.numWorkers
			case <-time.After(100 * time.Millisecond):
				slog.Warn("Worker pool full, dropping data", "worker", workerIndex)
				workerIndex = (workerIndex + 1) % e.numWorkers
			case <-e.ctx.Done():
				return
			}

		case <-e.ctx.Done():
			slog.Info("Distributor stopped", "distributed", distributedCount)
			return
		}
	}
}

// Worker: Processes individual market data
func (e *ExchangeService) worker(id int, workerChan <-chan domain.MarketData) {
	defer e.wg.Done()

	slog.Debug("Worker started", "id", id)
	defer slog.Debug("Worker stopped", "id", id)

	processedCount := 0

	for {
		select {
		case data, ok := <-workerChan:
			if !ok {
				slog.Debug("Worker channel closed", "id", id, "processed", processedCount)
				return
			}

			// Process the data
			processedData := e.processMarketData(data)

			// Send to result channel
			select {
			case e.resultChan <- processedData:
				processedCount++
				if processedCount%50 == 0 {
					slog.Debug("Worker stats", "id", id, "processed", processedCount)
				}
			case <-time.After(100 * time.Millisecond):
				slog.Warn("Result channel full, dropping processed data", "worker", id)
			case <-e.ctx.Done():
				return
			}

		case <-e.ctx.Done():
			slog.Debug("Worker context cancelled", "id", id, "processed", processedCount)
			return
		}
	}
}

// processMarketData validates and enriches market data
func (e *ExchangeService) processMarketData(data domain.MarketData) domain.MarketData {
	// Validate required fields
	if data.Symbol == "" || data.Price <= 0 {
		slog.Warn("Invalid market data", "symbol", data.Symbol, "price", data.Price, "exchange", data.Exchange)
		return data
	}

	// Validate symbol is supported
	if !exchanges.IsSymbolSupported(data.Symbol) {
		slog.Warn("Unsupported symbol", "symbol", data.Symbol, "exchange", data.Exchange)
		return data
	}

	// Ensure timestamp is set
	if data.Timestamp == 0 {
		data.Timestamp = time.Now().Unix()
	}

	// Validate price range
	if data.Price > 1000000 || data.Price < 0.0001 {
		slog.Warn("Price out of expected range", "symbol", data.Symbol, "price", data.Price, "exchange", data.Exchange)
	}

	return data
}

// stopCurrentAdapters stops all currently active adapters
func (e *ExchangeService) stopCurrentAdapters() error {
	var errors []error

	for _, adapter := range e.activeAdapters {
		if err := adapter.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop adapter %s: %w", adapter.Name(), err))
		} else {
			slog.Info("Stopped adapter", "name", adapter.Name())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple errors stopping adapters: %v", errors)
	}

	return nil
}

// GetActiveAdapters returns the currently active adapters
func (e *ExchangeService) GetActiveAdapters() []port.ExchangeAdapter {
	e.modeMutex.RLock()
	defer e.modeMutex.RUnlock()

	result := make([]port.ExchangeAdapter, len(e.activeAdapters))
	copy(result, e.activeAdapters)
	return result
}

// IsRunning returns whether data processing is currently running
func (e *ExchangeService) IsRunning() bool {
	e.runMutex.RLock()
	defer e.runMutex.RUnlock()
	return e.isRunning
}

// GetStats returns basic statistics about the service
func (e *ExchangeService) GetStats() map[string]interface{} {
	e.modeMutex.RLock()
	e.runMutex.RLock()
	defer e.modeMutex.RUnlock()
	defer e.runMutex.RUnlock()

	healthyAdapters := 0
	adapterDetails := make([]map[string]interface{}, 0)

	for _, adapter := range e.activeAdapters {
		isHealthy := adapter.IsHealthy()
		if isHealthy {
			healthyAdapters++
		}

		adapterDetails = append(adapterDetails, map[string]interface{}{
			"name":    adapter.Name(),
			"healthy": isHealthy,
		})
	}

	return map[string]interface{}{
		"current_mode":      e.currentMode,
		"is_running":        e.isRunning,
		"active_adapters":   len(e.activeAdapters),
		"healthy_adapters":  healthyAdapters,
		"adapter_details":   adapterDetails,
		"num_workers":       e.numWorkers,
		"aggregated_buffer": len(e.aggregatedDataChan),
		"result_buffer":     len(e.resultChan),
	}
}
