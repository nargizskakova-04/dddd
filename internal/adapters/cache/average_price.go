package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cryptomarket/internal/core/domain"
)

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

	for _, exchange := range exchanges {
		prices, err := r.GetPricesInRangeByExchange(ctx, symbol, exchange, from, to)
		if err != nil {
			slog.Debug("No prices found for exchange in range",
				"exchange", exchange,
				"symbol", symbol,
				"error", err)
			continue
		}

		if len(prices) == 0 {
			slog.Debug("Empty price data for exchange",
				"exchange", exchange,
				"symbol", symbol)
			continue
		}

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

	averagePrice := calculateAveragePrice(prices)

	slog.Info("Found average price in cache for exchange",
		"symbol", symbol,
		"exchange", exchange,
		"price", averagePrice.Price,
		"timestamp", time.UnixMilli(averagePrice.Timestamp).Format(time.RFC3339),
		"data_points_checked", len(prices))

	return averagePrice, nil
}

func calculateAveragePrice(prices []domain.MarketData) *domain.MarketData {
	if len(prices) == 0 {
		return nil
	}

	var sum float64
	var latestTimestamp int64
	var latestExchange string

	for _, price := range prices {
		sum += price.Price
		if price.Timestamp > latestTimestamp {
			latestTimestamp = price.Timestamp
			latestExchange = price.Exchange
		}
	}

	averagePrice := sum / float64(len(prices))

	return &domain.MarketData{
		Symbol:    prices[0].Symbol,
		Price:     averagePrice,
		Timestamp: latestTimestamp,
		Exchange:  latestExchange,
	}
}

func getAvgPriceFromData(prices []domain.MarketData) float64 {
	if len(prices) == 0 {
		return 0
	}

	var sum float64
	for _, price := range prices {
		sum += price.Price
	}

	return sum / float64(len(prices))
}
