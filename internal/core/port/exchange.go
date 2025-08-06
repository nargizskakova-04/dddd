package port

import (
	"context"

	"cryptomarket/internal/core/domain"
)

type ExchangeAdapter interface {
	Start(ctx context.Context) (<-chan domain.MarketData, error)

	Stop() error

	Name() string

	IsHealthy() bool
}

type ExchangeService interface {
	SwitchToLiveMode(ctx context.Context) error

	SwitchToTestMode(ctx context.Context) error

	SwitchToAllMode(ctx context.Context) error

	GetCurrentMode() string

	GetModeExchanges() []string

	IsExchangeInCurrentMode(exchangeName string) bool

	StartDataProcessing(ctx context.Context) error

	StopDataProcessing() error

	GetDataStream() <-chan domain.MarketData

	GetAllAdapters() []ExchangeAdapter

	GetLiveAdapters() []ExchangeAdapter

	GetTestAdapters() []ExchangeAdapter

	IsRunning() bool

	GetStats() map[string]interface{}
}
