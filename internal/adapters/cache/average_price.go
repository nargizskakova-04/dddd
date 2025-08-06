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

	var allPrices []domain.MarketData

	// Collect prices from all specified exchanges
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

		// Add all prices from this exchange
		allPrices = append(allPrices, prices...)

		slog.Debug("Processed exchange data",
			"exchange", exchange,
			"data_points", len(prices))
	}

	if len(allPrices) == 0 {
		slog.Debug("No price data found in cache for any exchange",
			"symbol", symbol,
			"exchanges", exchanges)
		return nil, fmt.Errorf("no price data found for symbol %s from exchanges %v in time range", symbol, exchanges)
	}

	// Calculate the actual average price
	averagePrice := calculateAveragePrice(allPrices)

	slog.Info("Found average price in cache",
		"symbol", symbol,
		"price", averagePrice.Price,
		"exchange", averagePrice.Exchange,
		"timestamp", time.UnixMilli(averagePrice.Timestamp).Format(time.RFC3339),
		"data_points_checked", len(allPrices),
		"exchanges_checked", len(exchanges))

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

	// Calculate the actual average price
	averagePrice := calculateAveragePrice(prices)

	slog.Info("Found average price in cache for exchange",
		"symbol", symbol,
		"exchange", exchange,
		"price", averagePrice.Price,
		"timestamp", time.UnixMilli(averagePrice.Timestamp).Format(time.RFC3339),
		"data_points_checked", len(prices))

	return averagePrice, nil
}

// FIXED: Helper function to calculate the actual average price from a slice of MarketData
func calculateAveragePrice(prices []domain.MarketData) *domain.MarketData {
	if len(prices) == 0 {
		return nil
	}

	var sum float64
	var latestTimestamp int64
	var latestExchange string

	// Calculate sum and find the most recent data point for metadata
	for _, price := range prices {
		sum += price.Price
		if price.Timestamp > latestTimestamp {
			latestTimestamp = price.Timestamp
			latestExchange = price.Exchange
		}
	}

	// Calculate actual average
	averagePrice := sum / float64(len(prices))

	return &domain.MarketData{
		Symbol:    prices[0].Symbol, // All prices should have the same symbol
		Price:     averagePrice,     // FIXED: Now using actual average
		Timestamp: latestTimestamp,  // Use timestamp from most recent data point
		Exchange:  latestExchange,   // Use exchange from most recent data point
	}
}

// FIXED: Helper function to get average price from a slice of MarketData (for logging)
func getAvgPriceFromData(prices []domain.MarketData) float64 {
	if len(prices) == 0 {
		return 0
	}

	var sum float64
	for _, price := range prices {
		sum += price.Price
	}

	return sum / float64(len(prices)) // FIXED: Now returns actual average
}
