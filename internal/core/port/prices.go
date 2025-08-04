// Update internal/core/port/prices.go

package port

import (
	"context"
	"time"

	"cryptomarket/internal/core/domain"
)

// REMOVE the old PriceRepository interface and replace with this:
type PriceRepository interface {
	// Existing aggregation methods
	InsertAggregatedPrice(ctx context.Context, aggregatedPrice domain.Prices) error
	InsertAggregatedPrices(ctx context.Context, aggregatedPrices []domain.Prices) error
	GetLatestAggregationTime(ctx context.Context) (time.Time, error)
	GetAggregatedPricesInRange(ctx context.Context, symbol, exchange string, from, to time.Time) ([]domain.Prices, error)
	HealthCheck(ctx context.Context) error

	// NEW: Methods for highest prices from last 30 records
	GetHighestPriceFromLatestRecords(ctx context.Context, symbol string, allowedExchanges []string) (*domain.MarketData, error)
	GetHighestPriceByExchangeFromLatestRecord(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)
}

type PriceService interface {
	// Get the latest price for a symbol across all exchanges
	GetLatestPrice(ctx context.Context, symbol string) (*domain.MarketData, error)

	// Get the latest price for a symbol from a specific exchange
	GetLatestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)

	// New highest price methods
	GetHighestPrice(ctx context.Context, symbol string) (*domain.MarketData, error)
	GetHighestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)
}
