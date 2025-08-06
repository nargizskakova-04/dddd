// internal/adapters/cache/lowest_price.go
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cryptomarket/internal/core/domain"
)

// GetLowestPriceInRange finds the lowest price for a symbol across all allowed exchanges within a time range
func (r *RedisAdapter) GetLowestPriceInRange(ctx context.Context, symbol string, exchanges []string, from, to time.Time) (*domain.MarketData, error) {
	if len(exchanges) == 0 {
		return nil, fmt.Errorf("no exchanges specified")
	}

	slog.Debug("Getting lowest price in range from cache",
		"symbol", symbol,
		"exchanges", exchanges,
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339),
		"duration", to.Sub(from))

	var lowestPrice *domain.MarketData
	var minPrice float64 // FIXED: Changed from maxPrice to minPrice

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

		// Find lowest price in this exchange's data
		for _, price := range prices {
			// FIXED: Proper lowest price logic
			if lowestPrice == nil || price.Price < minPrice {
				minPrice = price.Price
				lowestPrice = &domain.MarketData{
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
			"exchange_min_price", getMinPriceFromData(prices)) // FIXED: Changed function name
	}

	if lowestPrice == nil {
		slog.Debug("No price data found in cache for any exchange",
			"symbol", symbol,
			"exchanges", exchanges)
		return nil, fmt.Errorf("no price data found for symbol %s from exchanges %v in time range", symbol, exchanges)
	}

	slog.Info("Found lowest price in cache",
		"symbol", symbol,
		"price", lowestPrice.Price,
		"exchange", lowestPrice.Exchange,
		"timestamp", time.UnixMilli(lowestPrice.Timestamp).Format(time.RFC3339),
		"data_points_checked", "multiple_exchanges")

	return lowestPrice, nil
}

// GetLowestPriceInRangeByExchange finds the lowest price for a symbol from a specific exchange within a time range
func (r *RedisAdapter) GetLowestPriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error) {
	slog.Debug("Getting lowest price in range by exchange from cache",
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

	// Find the lowest price
	lowestPrice := &prices[0]
	minPrice := prices[0].Price // FIXED: Changed from maxPrice to minPrice

	for i := 1; i < len(prices); i++ {
		// FIXED: Proper lowest price comparison
		if prices[i].Price < minPrice {
			minPrice = prices[i].Price
			lowestPrice = &prices[i]
		}
	}

	slog.Info("Found lowest price in cache for exchange",
		"symbol", symbol,
		"exchange", exchange,
		"price", lowestPrice.Price,
		"timestamp", time.UnixMilli(lowestPrice.Timestamp).Format(time.RFC3339),
		"data_points_checked", len(prices))

	return lowestPrice, nil
}

// FIXED: Helper function to get min price from a slice of MarketData
func getMinPriceFromData(prices []domain.MarketData) float64 {
	if len(prices) == 0 {
		return 0
	}

	minPrice := prices[0].Price
	for _, price := range prices[1:] {
		if price.Price < minPrice { // FIXED: Changed from > to <
			minPrice = price.Price
		}
	}
	return minPrice
}
