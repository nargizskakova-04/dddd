// internal/core/port/cache.go
package port

import (
	"context"
	"time"

	"cryptomarket/internal/core/domain"
)

// Update internal/core/port/cache.go - ADD these methods to the existing Cache interface
type Cache interface {
	// Existing methods
	SetPrice(ctx context.Context, key string, data domain.MarketData) error
	GetLatestPrice(ctx context.Context, symbol string) (*domain.MarketData, error)
	GetLatestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)
	GetLatestPriceFromExchanges(ctx context.Context, symbol string, exchanges []string) (*domain.MarketData, error)
	GetPricesInRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.MarketData, error)
	GetPricesInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) ([]domain.MarketData, error)
	GetPricesInRangeFromExchanges(ctx context.Context, symbol string, exchanges []string, from, to time.Time) ([]domain.MarketData, error)
	CleanupOldData(ctx context.Context, olderThan time.Duration) error
	Ping(ctx context.Context) error

	// NEW: Highest price methods with time range support
	GetHighestPriceInRange(ctx context.Context, symbol string, exchanges []string, from, to time.Time) (*domain.MarketData, error)
	GetHighestPriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error)
	GetLatestPricesCount(ctx context.Context, symbol, exchange string, count int) ([]domain.MarketData, error)
}
