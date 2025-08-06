package exchange

import (
	"context"
	"log/slog"
)

func (e *ExchangeService) SwitchToLiveMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == ModeLive {
		slog.Info("Already in live mode")
		return nil
	}

	previousMode := e.currentMode
	e.currentMode = ModeLive

	slog.Info("Switched to live mode", "previous_mode", previousMode, "current_mode", e.currentMode)
	slog.Info("Live mode will filter data to exchanges: exchange1, exchange2, exchange3")

	return nil
}

func (e *ExchangeService) SwitchToTestMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == ModeTest {
		slog.Info("Already in test mode")
		return nil
	}

	previousMode := e.currentMode
	e.currentMode = ModeTest

	slog.Info("Switched to test mode", "previous_mode", previousMode, "current_mode", e.currentMode)
	slog.Info("Test mode will filter data to exchanges: test-exchange1, test-exchange2, test-exchange3")

	return nil
}

func (e *ExchangeService) SwitchToAllMode(ctx context.Context) error {
	e.modeMutex.Lock()
	defer e.modeMutex.Unlock()

	if e.currentMode == ModeAll {
		slog.Info("Already in all mode")
		return nil
	}

	previousMode := e.currentMode
	e.currentMode = ModeAll

	slog.Info("Switched to all mode", "previous_mode", previousMode, "current_mode", e.currentMode)
	slog.Info("All mode will use data from all 6 exchanges")

	return nil
}

func (e *ExchangeService) GetCurrentMode() string {
	e.modeMutex.RLock()
	defer e.modeMutex.RUnlock()
	return e.currentMode
}

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

		return []string{"exchange1", "exchange2", "exchange3"}
	}
}

func (e *ExchangeService) IsExchangeInCurrentMode(exchangeName string) bool {
	allowedExchanges := e.GetModeExchanges()

	for _, allowed := range allowedExchanges {
		if allowed == exchangeName {
			return true
		}
	}

	return false
}
