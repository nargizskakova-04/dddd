// internal/core/port/exchange.go
package port

import (
	"context"

	"cryptomarket/internal/core/domain"
)

type ExchangeAdapter interface {
	// Start streaming market data
	Start(ctx context.Context) (<-chan domain.MarketData, error)

	// Stop streaming
	Stop() error

	// Get exchange name/identifier
	Name() string

	// Health check
	IsHealthy() bool
}

type ExchangeService interface {
	// Switch to live mode (only affects reading from Redis, not data collection)
	SwitchToLiveMode(ctx context.Context) error

	// Switch to test mode (only affects reading from Redis, not data collection)
	SwitchToTestMode(ctx context.Context) error

	// NEW: Switch to all mode (uses data from all exchanges)
	SwitchToAllMode(ctx context.Context) error

	// Get current mode
	GetCurrentMode() string

	// NEW: Get list of exchanges that should be used for the current mode
	GetModeExchanges() []string

	// NEW: Check if an exchange should be used in the current mode
	IsExchangeInCurrentMode(exchangeName string) bool

	// Start data processing (now always processes ALL exchanges)
	StartDataProcessing(ctx context.Context) error

	// Stop data processing
	StopDataProcessing() error

	// Get aggregated data channel (contains data from all exchanges)
	GetDataStream() <-chan domain.MarketData

	// NEW: Get all adapters for monitoring
	GetAllAdapters() []ExchangeAdapter

	// NEW: Get live adapters
	GetLiveAdapters() []ExchangeAdapter

	// NEW: Get test adapters
	GetTestAdapters() []ExchangeAdapter

	// NEW: Check if data processing is running
	IsRunning() bool

	// NEW: Get service statistics
	GetStats() map[string]interface{}
}
