// internal/adapters/cache/average_price.go
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cryptomarket/internal/core/domain"
)

// GetAveragePriceInRange finds the average price for a symbol across all allowed exchanges within a time range
func (r *RedisAdapter) GetAveragePriceInRange(ctx context.Context, symbol string, exchanges []string, from, to time.Time) (*domain.MarketData, error) {
	if len(exchanges) == 0 {
		return nil, fmt.Errorf("no exchanges specified")
	}

	slog.Debug("Getting average price in range from cache",
		"symbol", symbol,
		"exchanges", exchanges,
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339),
		"duration", to.Sub(from))

	var averagePrice *domain.MarketData
	var avgPrice float64

	// Check each specified exchange
	for _, exchange := range exchanges {
		prices, err := r.GetPricesInRangeByExchange(ctx, symbol, exchange, from, to)
		if err != nil {
			slog.Debug("No prices found for exchange in range",
				"exchange", exchange,
				"symbol", symbol,
				"error", err)
			continue // Skip this exchange if no data or error
		}

		if len(prices) == 0 {
			slog.Debug("Empty price data for exchange",
				"exchange", exchange,
				"symbol", symbol)
			continue
		}

		// Find average price in this exchange's data
		for _, price := range prices {
			if averagePrice == nil || price.Price > avgPrice {
				avgPrice = price.Price
				averagePrice = &domain.MarketData{
					Symbol:    price.Symbol,
					Price:     price.Price,
					Timestamp: price.Timestamp,
					Exchange:  price.Exchange,
				}
			}
		}

		slog.Debug("Processed exchange data",
			"exchange", exchange,
			"data_points", len(prices),
			"exchange_avg_price", getAvgPriceFromData(prices))
	}

	if averagePrice == nil {
		slog.Debug("No price data found in cache for any exchange",
			"symbol", symbol,
			"exchanges", exchanges)
		return nil, fmt.Errorf("no price data found for symbol %s from exchanges %v in time range", symbol, exchanges)
	}

	slog.Info("Found average price in cache",
		"symbol", symbol,
		"price", averagePrice.Price,
		"exchange", averagePrice.Exchange,
		"timestamp", time.UnixMilli(averagePrice.Timestamp).Format(time.RFC3339),
		"data_points_checked", "multiple_exchanges")

	return averagePrice, nil
}

// GetAveragePriceInRangeByExchange finds the average price for a symbol from a specific exchange within a time range
func (r *RedisAdapter) GetAveragePriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error) {
	slog.Debug("Getting average price in range by exchange from cache",
		"symbol", symbol,
		"exchange", exchange,
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339),
		"duration", to.Sub(from))

	prices, err := r.GetPricesInRangeByExchange(ctx, symbol, exchange, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get prices in range: %w", err)
	}

	if len(prices) == 0 {
		slog.Debug("No price data found in cache for exchange",
			"symbol", symbol,
			"exchange", exchange)
		return nil, fmt.Errorf("no price data found for symbol %s from exchange %s in time range", symbol, exchange)
	}

	// Find the average price
	averagePrice := &prices[0]
	avgPrice := prices[0].Price

	for i := 1; i < len(prices); i++ {
		if prices[i].Price > avgPrice {
			avgPrice = prices[i].Price
			averagePrice = &prices[i]
		}
	}

	slog.Info("Found average price in cache for exchange",
		"symbol", symbol,
		"exchange", exchange,
		"price", averagePrice.Price,
		"timestamp", time.UnixMilli(averagePrice.Timestamp).Format(time.RFC3339),
		"data_points_checked", len(prices))

	return averagePrice, nil
}

// Helper function to get avg price from a slice of MarketData
func getAvgPriceFromData(prices []domain.MarketData) float64 {
	if len(prices) == 0 {
		return 0
	}

	avgPrice := prices[0].Price
	for _, price := range prices[1:] {
		if price.Price > avgPrice {
			avgPrice = price.Price
		}
	}
	return avgPrice
}
