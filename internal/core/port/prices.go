// Update internal/core/port/prices.go - ADD these methods to existing interfaces

package port

import (
	"context"
	"time"

	"cryptomarket/internal/core/domain"
)

type PriceRepository interface {
	InsertAggregatedPrice(ctx context.Context, aggregatedPrice domain.Prices) error
	InsertAggregatedPrices(ctx context.Context, aggregatedPrices []domain.Prices) error
	GetLatestAggregationTime(ctx context.Context) (time.Time, error)
	GetAggregatedPricesInRange(ctx context.Context, symbol, exchange string, from, to time.Time) ([]domain.Prices, error)
	HealthCheck(ctx context.Context) error

	GetHighestPriceFromLatestRecords(ctx context.Context, symbol string, allowedExchanges []string) (*domain.MarketData, error)
	GetHighestPriceByExchangeFromLatestRecord(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)
	GetLowestPriceFromLatestRecords(ctx context.Context, symbol string, allowedExchanges []string) (*domain.MarketData, error)
	GetLowestPriceByExchangeFromLatestRecord(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)

	GetHighestPriceInRange(ctx context.Context, symbol string, allowedExchanges []string, from, to time.Time) (*domain.MarketData, error)
	GetHighestPriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error)
	GetPriceDataStats(ctx context.Context, symbol string, exchanges []string) (map[string]interface{}, error)

	GetLowestPriceInRange(ctx context.Context, symbol string, allowedExchanges []string, from, to time.Time) (*domain.MarketData, error)
	GetLowestPriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error)

	GetAveragePriceInRange(ctx context.Context, symbol string, allowedExchanges []string, from, to time.Time) (*domain.MarketData, error)
	GetAveragePriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error)
	GetAveragePriceFromLatestRecords(ctx context.Context, symbol string, allowedExchanges []string) (*domain.MarketData, error)
	GetAveragePriceByExchangeFromLatestRecord(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)
}

type PriceService interface {
	GetLatestPrice(ctx context.Context, symbol string) (*domain.MarketData, error)
	GetLatestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)
	GetHighestPrice(ctx context.Context, symbol string) (*domain.MarketData, error)
	GetHighestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)
	GetLowestPrice(ctx context.Context, symbol string) (*domain.MarketData, error)
	GetLowestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)
	GetAveragePrice(ctx context.Context, symbol string) (*domain.MarketData, error)
	GetAveragePriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)

	GetHighestPriceWithPeriod(ctx context.Context, symbol, period string) (*domain.MarketData, error)
	GetHighestPriceByExchangeWithPeriod(ctx context.Context, symbol, exchange, period string) (*domain.MarketData, error)

	GetLowestPriceWithPeriod(ctx context.Context, symbol, period string) (*domain.MarketData, error)
	GetLowestPriceByExchangeWithPeriod(ctx context.Context, symbol, exchange, period string) (*domain.MarketData, error)

	GetAveragePriceByExchangeWithPeriod(ctx context.Context, symbol, exchange, periodStr string) (*domain.MarketData, error)
	GetAveragePriceWithPeriod(ctx context.Context, symbol, periodStr string) (*domain.MarketData, error)

	GetPeriodInfo(period string) (map[string]interface{}, error)
}
