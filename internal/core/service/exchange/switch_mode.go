// internal/core/service/exchange/switch_mode.go
package exchange

import (
	"context"
	"log/slog"
)

// SwitchToLiveMode switches to live mode (only affects reading from Redis)
func (e *ExchangeService) SwitchToLiveMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == ModeLive {
		slog.Info("Already in live mode")
		return nil // Already in live mode
	}

	previousMode := e.currentMode
	e.currentMode = ModeLive

	slog.Info("Switched to live mode", "previous_mode", previousMode, "current_mode", e.currentMode)
	slog.Info("Live mode will filter data to exchanges: exchange1, exchange2, exchange3")

	return nil
}

// SwitchToTestMode switches to test mode (only affects reading from Redis)
func (e *ExchangeService) SwitchToTestMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == ModeTest {
		slog.Info("Already in test mode")
		return nil // Already in test mode
	}

	previousMode := e.currentMode
	e.currentMode = ModeTest

	slog.Info("Switched to test mode", "previous_mode", previousMode, "current_mode", e.currentMode)
	slog.Info("Test mode will filter data to exchanges: test-exchange1, test-exchange2, test-exchange3")

	return nil
}

// SwitchToAllMode switches to all mode (uses data from all exchanges)
func (e *ExchangeService) SwitchToAllMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == ModeAll {
		slog.Info("Already in all mode")
		return nil // Already in all mode
	}

	previousMode := e.currentMode
	e.currentMode = ModeAll

	slog.Info("Switched to all mode", "previous_mode", previousMode, "current_mode", e.currentMode)
	slog.Info("All mode will use data from all 6 exchanges")

	return nil
}

// GetCurrentMode returns the current mode
func (e *ExchangeService) GetCurrentMode() string {
	e.modeMutex.RLock()
	defer e.modeMutex.RUnlock()
	return e.currentMode
}

// GetModeExchanges returns the list of exchange names that should be used for the current mode
func (e *ExchangeService) GetModeExchanges() []string {
	e.modeMutex.RLock()
	defer e.modeMutex.RUnlock()

	switch e.currentMode {
	case ModeLive:
		return []string{"exchange1", "exchange2", "exchange3"}
	case ModeTest:
		return []string{"test-exchange1", "test-exchange2", "test-exchange3"}
	case ModeAll:
		return []string{"exchange1", "exchange2", "exchange3", "test-exchange1", "test-exchange2", "test-exchange3"}
	default:
		// Default to live mode exchanges
		return []string{"exchange1", "exchange2", "exchange3"}
	}
}

// IsExchangeInCurrentMode checks if an exchange should be used in the current mode
func (e *ExchangeService) IsExchangeInCurrentMode(exchangeName string) bool {
	allowedExchanges := e.GetModeExchanges()

	for _, allowed := range allowedExchanges {
		if allowed == exchangeName {
			return true
		}
	}

	return false
}
