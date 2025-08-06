// internal/core/service/exchange/service.go
package exchange

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"cryptomarket/internal/adapters/exchanges"
	"cryptomarket/internal/core/domain"
	"cryptomarket/internal/core/port"
)

const (
	ModeLive = "live"
	ModeTest = "test"
	ModeAll  = "all"
)

// ExchangeService now always processes data from all exchanges
// Mode only affects which exchanges are considered when reading data
type ExchangeService struct {
	// Mode management - only affects reading, not data collection
	currentMode string
	modeMutex   sync.RWMutex

	// All exchange adapters - always active
	liveAdapters []port.ExchangeAdapter
	testAdapters []port.ExchangeAdapter
	allAdapters  []port.ExchangeAdapter // Combined list for easy iteration

	// Concurrency channels
	aggregatedDataChan chan domain.MarketData   // Fan-in result
	workerPool         []chan domain.MarketData // Fan-out to workers
	resultChan         chan domain.MarketData   // Final processed data

	// Control
	ctx       context.Context
	cancel    context.CancelFunc
	isRunning bool
	runMutex  sync.RWMutex
	wg        sync.WaitGroup

	// Configuration
	numWorkers int
}

// NewExchangeService creates a new exchange service that always processes all exchanges
func NewExchangeService() port.ExchangeService {
	ctx, cancel := context.WithCancel(context.Background())

	// Create live adapters for exchanges on ports 40101, 40102, 40103
	liveAdapters := exchanges.CreateLiveExchangeAdapters()

	// Create test adapters (generators)
	testAdapters := exchanges.CreateTestExchangeAdapters()

	// Combine all adapters
	allAdapters := make([]port.ExchangeAdapter, 0, len(liveAdapters)+len(testAdapters))
	allAdapters = append(allAdapters, liveAdapters...)
	allAdapters = append(allAdapters, testAdapters...)

	numWorkers := 30 // Increased to handle all 6 exchanges (5 workers per exchange)

	return &ExchangeService{
		currentMode:        ModeLive, // Default to live mode
		liveAdapters:       liveAdapters,
		testAdapters:       testAdapters,
		allAdapters:        allAdapters,
		aggregatedDataChan: make(chan domain.MarketData, 2000), // Increased buffer for more data
		workerPool:         make([]chan domain.MarketData, numWorkers),
		resultChan:         make(chan domain.MarketData, 2000), // Increased buffer
		ctx:                ctx,
		cancel:             cancel,
		numWorkers:         numWorkers,
	}
}

// NewExchangeServiceWithAdapters creates a service with custom adapters
func NewExchangeServiceWithAdapters(liveAdapters, testAdapters []port.ExchangeAdapter) port.ExchangeService {
	ctx, cancel := context.WithCancel(context.Background())

	// Combine all adapters
	allAdapters := make([]port.ExchangeAdapter, 0, len(liveAdapters)+len(testAdapters))
	allAdapters = append(allAdapters, liveAdapters...)
	allAdapters = append(allAdapters, testAdapters...)

	numWorkers := 30 // Handle all exchanges

	return &ExchangeService{
		currentMode:        ModeLive, // Default to live mode
		liveAdapters:       liveAdapters,
		testAdapters:       testAdapters,
		allAdapters:        allAdapters,
		aggregatedDataChan: make(chan domain.MarketData, 2000),
		workerPool:         make([]chan domain.MarketData, numWorkers),
		resultChan:         make(chan domain.MarketData, 2000),
		ctx:                ctx,
		cancel:             cancel,
		numWorkers:         numWorkers,
	}
}

// StartDataProcessing now always starts ALL exchange adapters
func (e *ExchangeService) StartDataProcessing(ctx context.Context) error {
	e.runMutex.Lock()
	defer e.runMutex.Unlock()

	if e.isRunning {
		return nil // Already running
	}

	slog.Info("Starting data processing for ALL exchanges...", "mode", e.currentMode, "workers", e.numWorkers, "total_exchanges", len(e.allAdapters))

	// Initialize worker pool channels
	for i := 0; i < e.numWorkers; i++ {
		e.workerPool[i] = make(chan domain.MarketData, 100)
	}

	// Start ALL exchange adapters and collect their channels
	var inputChannels []<-chan domain.MarketData
	for _, adapter := range e.allAdapters {
		dataChan, err := adapter.Start(e.ctx)
		if err != nil {
			slog.Error("Failed to start adapter", "adapter", adapter.Name(), "error", err)
			continue
		}
		inputChannels = append(inputChannels, dataChan)
		slog.Info("Started exchange adapter", "name", adapter.Name(), "healthy", adapter.IsHealthy())
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
	slog.Info("Data processing started successfully", "exchanges", len(inputChannels), "workers", e.numWorkers)
	return nil
}

// StopDataProcessing stops all adapters
func (e *ExchangeService) StopDataProcessing() error {
	e.runMutex.Lock()
	defer e.runMutex.Unlock()

	if !e.isRunning {
		return nil // Already stopped
	}

	slog.Info("Stopping data processing...")

	// Stop ALL adapters
	if err := e.stopAllAdapters(); err != nil {
		slog.Error("Failed to stop adapters", "error", err)
	}

	// Cancel context to stop all goroutines
	e.cancel()

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		slog.Info("All goroutines stopped")
	case <-time.After(10 * time.Second):
		slog.Warn("Timeout waiting for goroutines to stop")
	}

	// Close worker channels
	for _, workerChan := range e.workerPool {
		close(workerChan)
	}

	// Close result channel
	close(e.resultChan)

	// Recreate context and channels for next start
	e.ctx, e.cancel = context.WithCancel(context.Background())
	e.aggregatedDataChan = make(chan domain.MarketData, 2000)
	e.resultChan = make(chan domain.MarketData, 2000)
	for i := range e.workerPool {
		e.workerPool[i] = make(chan domain.MarketData, 100)
	}

	e.isRunning = false
	slog.Info("Data processing stopped")
	return nil
}

// stopAllAdapters stops all adapters (both live and test)
func (e *ExchangeService) stopAllAdapters() error {
	var errors []error

	for _, adapter := range e.allAdapters {
		if err := adapter.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop adapter %s: %w", adapter.Name(), err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple errors stopping adapters: %v", errors)
	}

	return nil
}

// GetDataStream returns the channel with all processed data
func (e *ExchangeService) GetDataStream() <-chan domain.MarketData {
	return e.resultChan
}

// Fan-in: Aggregates data from all exchange channels into one
func (e *ExchangeService) fanIn(inputChannels []<-chan domain.MarketData) {
	defer e.wg.Done()
	defer close(e.aggregatedDataChan)

	slog.Info("Starting fan-in aggregator for ALL exchanges", "inputs", len(inputChannels))

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

	for {
		select {
		case data, ok := <-e.aggregatedDataChan:
			if !ok {
				slog.Info("Aggregated data channel closed")
				return
			}

			// Round-robin distribution to workers
			select {
			case e.workerPool[workerIndex] <- data:
				workerIndex = (workerIndex + 1) % e.numWorkers
			case <-time.After(100 * time.Millisecond):
				slog.Warn("Worker pool full, dropping data", "worker", workerIndex)
				workerIndex = (workerIndex + 1) % e.numWorkers
			case <-e.ctx.Done():
				return
			}

		case <-e.ctx.Done():
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

			// Process the data (validation, enrichment, etc.)
			processedData := e.processMarketData(data)

			// Send to result channel
			select {
			case e.resultChan <- processedData:
				processedCount++
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

	// Validate price range (basic sanity check)
	if data.Price > 1000000 || data.Price < 0.0001 {
		slog.Warn("Price out of expected range", "symbol", data.Symbol, "price", data.Price, "exchange", data.Exchange)
	}

	return data
}

// GetAllAdapters returns all adapters (for monitoring)
func (e *ExchangeService) GetAllAdapters() []port.ExchangeAdapter {
	return e.allAdapters
}

// GetLiveAdapters returns live adapters
func (e *ExchangeService) GetLiveAdapters() []port.ExchangeAdapter {
	return e.liveAdapters
}

// GetTestAdapters returns test adapters
func (e *ExchangeService) GetTestAdapters() []port.ExchangeAdapter {
	return e.testAdapters
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
	for _, adapter := range e.allAdapters {
		if adapter.IsHealthy() {
			healthyAdapters++
		}
	}

	return map[string]interface{}{
		"current_mode":      e.currentMode,
		"is_running":        e.isRunning,
		"total_adapters":    len(e.allAdapters),
		"live_adapters":     len(e.liveAdapters),
		"test_adapters":     len(e.testAdapters),
		"healthy_adapters":  healthyAdapters,
		"num_workers":       e.numWorkers,
		"aggregated_buffer": len(e.aggregatedDataChan),
		"result_buffer":     len(e.resultChan),
	}
}
