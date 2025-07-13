package port

import (
	"context"
	"time"

	"crypto/internal/core/domain"
)

type Cache interface {
	// Store price data with timestamp
	SetPrice(ctx context.Context, key string, data domain.MarketData) error

	// Get latest price for a symbol
	GetLatestPrice(ctx context.Context, symbol string) (*domain.MarketData, error)

	// Get latest price for a symbol from specific exchange
	GetLatestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)

	// Get all prices for a symbol within time range
	GetPricesInRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.MarketData, error)

	// Get all prices for a symbol from specific exchange within time range
	GetPricesInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) ([]domain.MarketData, error)

	// Clean up old data (older than specified duration)
	CleanupOldData(ctx context.Context, olderThan time.Duration) error

	// Health check
	Ping(ctx context.Context) error
}
