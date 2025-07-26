package exchange

import (
	"context"
	"fmt"
	"log/slog"
)

func (e *ExchangeService) SwitchToLiveMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == "live" {
		return nil // Already in live mode
	}

	slog.Info("Switching to live mode...")

	// Stop current adapters
	if err := e.stopCurrentAdapters(); err != nil {
		return fmt.Errorf("failed to stop current adapters: %w", err)
	}

	// Switch to live adapters
	e.activeAdapters = e.liveAdapters
	e.currentMode = "live"

	// Restart data processing if it was running
	e.runMutex.RLock()
	wasRunning := e.isRunning
	e.runMutex.RUnlock()

	if wasRunning {
		if err := e.StartDataProcessing(ctx); err != nil {
			return fmt.Errorf("failed to restart data processing: %w", err)
		}
	}

	slog.Info("Switched to live mode successfully")
	return nil
}

func (e *ExchangeService) SwitchToTestMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == "test" {
		return nil // Already in test mode
	}

	slog.Info("Switching to test mode...")

	// Stop current adapters
	if err := e.stopCurrentAdapters(); err != nil {
		return fmt.Errorf("failed to stop current adapters: %w", err)
	}

	// Switch to test adapters
	e.activeAdapters = e.testAdapters
	e.currentMode = "test"

	// Restart data processing if it was running
	e.runMutex.RLock()
	wasRunning := e.isRunning
	e.runMutex.RUnlock()

	if wasRunning {
		if err := e.StartDataProcessing(ctx); err != nil {
			return fmt.Errorf("failed to restart data processing: %w", err)
		}
	}

	slog.Info("Switched to test mode successfully")
	return nil
}

func (e *ExchangeService) GetCurrentMode() string {
	e.modeMutex.RLock()
	defer e.modeMutex.RUnlock()
	return e.currentMode
}
