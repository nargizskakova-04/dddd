package port

import (
	"context"

	"crypto/internal/core/domain"
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
	// Switch to live mode (connect to real exchanges)
	SwitchToLiveMode(ctx context.Context) error

	// Switch to test mode (use synthetic data)
	SwitchToTestMode(ctx context.Context) error

	// Get current mode
	GetCurrentMode() string

	// Start data processing
	StartDataProcessing(ctx context.Context) error

	// Stop data processing
	StopDataProcessing() error

	// Get aggregated data channel
	GetDataStream() <-chan domain.MarketData
}
