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

type ExchangeService struct {
	currentMode string
	modeMutex   sync.RWMutex

	liveAdapters []port.ExchangeAdapter
	testAdapters []port.ExchangeAdapter
	allAdapters  []port.ExchangeAdapter

	aggregatedDataChan chan domain.MarketData
	workerPool         []chan domain.MarketData
	resultChan         chan domain.MarketData

	ctx       context.Context
	cancel    context.CancelFunc
	isRunning bool
	runMutex  sync.RWMutex
	wg        sync.WaitGroup

	numWorkers int
}

func NewExchangeService() port.ExchangeService {
	ctx, cancel := context.WithCancel(context.Background())

	liveAdapters := exchanges.CreateLiveExchangeAdapters()

	testAdapters := exchanges.CreateTestExchangeAdapters()

	allAdapters := make([]port.ExchangeAdapter, 0, len(liveAdapters)+len(testAdapters))
	allAdapters = append(allAdapters, liveAdapters...)
	allAdapters = append(allAdapters, testAdapters...)

	numWorkers := 30

	return &ExchangeService{
		currentMode:        ModeLive,
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

func NewExchangeServiceWithAdapters(liveAdapters, testAdapters []port.ExchangeAdapter) port.ExchangeService {
	ctx, cancel := context.WithCancel(context.Background())

	allAdapters := make([]port.ExchangeAdapter, 0, len(liveAdapters)+len(testAdapters))
	allAdapters = append(allAdapters, liveAdapters...)
	allAdapters = append(allAdapters, testAdapters...)

	numWorkers := 30

	return &ExchangeService{
		currentMode:        ModeLive,
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

func (e *ExchangeService) StartDataProcessing(ctx context.Context) error {
	e.runMutex.Lock()
	defer e.runMutex.Unlock()

	if e.isRunning {
		return nil
	}

	slog.Info("Starting data processing for ALL exchanges...", "mode", e.currentMode, "workers", e.numWorkers, "total_exchanges", len(e.allAdapters))

	for i := 0; i < e.numWorkers; i++ {
		e.workerPool[i] = make(chan domain.MarketData, 100)
	}

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

func (e *ExchangeService) StopDataProcessing() error {
	e.runMutex.Lock()
	defer e.runMutex.Unlock()

	if !e.isRunning {
		return nil
	}

	slog.Info("Stopping data processing...")

	if err := e.stopAllAdapters(); err != nil {
		slog.Error("Failed to stop adapters", "error", err)
	}

	e.cancel()

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

	for _, workerChan := range e.workerPool {
		close(workerChan)
	}

	close(e.resultChan)

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

func (e *ExchangeService) GetDataStream() <-chan domain.MarketData {
	return e.resultChan
}

func (e *ExchangeService) fanIn(inputChannels []<-chan domain.MarketData) {
	defer e.wg.Done()
	defer close(e.aggregatedDataChan)

	slog.Info("Starting fan-in aggregator for ALL exchanges", "inputs", len(inputChannels))

	var fanInWg sync.WaitGroup

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

			processedData := e.processMarketData(data)

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

func (e *ExchangeService) processMarketData(data domain.MarketData) domain.MarketData {

	if data.Symbol == "" || data.Price <= 0 {
		slog.Warn("Invalid market data", "symbol", data.Symbol, "price", data.Price, "exchange", data.Exchange)
		return data
	}

	if !exchanges.IsSymbolSupported(data.Symbol) {
		slog.Warn("Unsupported symbol", "symbol", data.Symbol, "exchange", data.Exchange)
		return data
	}

	if data.Timestamp == 0 {
		data.Timestamp = time.Now().Unix()
	}

	if data.Price > 1000000 || data.Price < 0.0001 {
		slog.Warn("Price out of expected range", "symbol", data.Symbol, "price", data.Price, "exchange", data.Exchange)
	}

	return data
}

func (e *ExchangeService) GetAllAdapters() []port.ExchangeAdapter {
	return e.allAdapters
}

func (e *ExchangeService) GetLiveAdapters() []port.ExchangeAdapter {
	return e.liveAdapters
}

func (e *ExchangeService) GetTestAdapters() []port.ExchangeAdapter {
	return e.testAdapters
}

func (e *ExchangeService) IsRunning() bool {
	e.runMutex.RLock()
	defer e.runMutex.RUnlock()
	return e.isRunning
}

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
