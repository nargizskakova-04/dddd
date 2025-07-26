package exchange

import (
	"context"
	"cryptomarket/internal/core/domain"
	"fmt"
	"log/slog"
	"time"
)

func (e *ExchangeService) StartDataProcessing(ctx context.Context) error {
	e.runMutex.Lock()
	defer e.runMutex.Unlock()

	if e.isRunning {
		return nil // Already running
	}

	slog.Info("Starting data processing...", "mode", e.currentMode, "workers", e.numWorkers)

	// Initialize worker pool channels
	for i := 0; i < e.numWorkers; i++ {
		e.workerPool[i] = make(chan domain.MarketData, 100)
	}

	// Start exchange adapters and collect their channels
	var inputChannels []<-chan domain.MarketData
	for _, adapter := range e.activeAdapters {
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

func (e *ExchangeService) StopDataProcessing() error {
	e.runMutex.Lock()
	defer e.runMutex.Unlock()

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
	e.aggregatedDataChan = make(chan domain.MarketData, 1000)
	e.resultChan = make(chan domain.MarketData, 1000)
	for i := range e.workerPool {
		e.workerPool[i] = make(chan domain.MarketData, 100)
	}

	e.isRunning = false
	slog.Info("Data processing stopped")
	return nil
}

func (e *ExchangeService) GetDataStream() <-chan domain.MarketData {
	return e.resultChan
}
