package port

import (
	"context"
	"time"

	"crypto/internal/core/domain"
)

type PriceRepository interface {
	GetLatestPrice(symbol string) (domain.GetPrice, error)
	GetLatestPriceByExchange(symbol string, exchange string) (domain.GetPrice, error)

	GetHighestPrice(symbol string) (domain.GetPrice, error)
	GetHighestPriceExchange(symbol string, exchange string) (domain.GetPrice, error)
	GetHighestPriceInDuration(symbol string, from time.Time, to time.Time) (domain.GetPrice, error)
	GetHighestPriceInDurationExchange(symbol string, exchange string, from time.Time, to time.Time) (domain.GetPrice, error)

	GetLowestPrice(symbol string) (domain.GetPrice, error)
	GetLowestPriceExchange(symbol string, exchange string) (domain.GetPrice, error)
	GetLowestPriceInDuration(symbol string, from time.Time, to time.Time) (domain.GetPrice, error)
	GetLowestPriceInDurationExchange(symbol string, exchange string, from time.Time, to time.Time) (domain.GetPrice, error)

	GetAveragePrice(symbol string) (domain.GetPrice, error)
	GetAveragePriceExchange(symbol string, exchange string) (domain.GetPrice, error)
	GetAveragePriceInDurationExchange(symbol string, exchange string, from time.Time, to time.Time) (domain.GetPrice, error)
}

type PriceService interface {
	// Get the latest price for a symbol across all exchanges
	GetLatestPrice(ctx context.Context, symbol string) (*domain.MarketData, error)

	// Get the latest price for a symbol from a specific exchange
	GetLatestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error)
}
