package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cryptomarket/internal/core/domain"
)

func (r *RedisAdapter) GetHighestPriceInRange(ctx context.Context, symbol string, exchanges []string, from, to time.Time) (*domain.MarketData, error) {
	if len(exchanges) == 0 {
		return nil, fmt.Errorf("no exchanges specified")
	}

	slog.Debug("Getting highest price in range from cache",
		"symbol", symbol,
		"exchanges", exchanges,
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339),
		"duration", to.Sub(from))

	var highestPrice *domain.MarketData
	var maxPrice float64

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

		for _, price := range prices {
			if highestPrice == nil || price.Price > maxPrice {
				maxPrice = price.Price
				highestPrice = &domain.MarketData{
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
			"exchange_max_price", getMaxPriceFromData(prices))
	}

	if highestPrice == nil {
		slog.Debug("No price data found in cache for any exchange",
			"symbol", symbol,
			"exchanges", exchanges)
		return nil, fmt.Errorf("no price data found for symbol %s from exchanges %v in time range", symbol, exchanges)
	}

	slog.Info("Found highest price in cache",
		"symbol", symbol,
		"price", highestPrice.Price,
		"exchange", highestPrice.Exchange,
		"timestamp", time.UnixMilli(highestPrice.Timestamp).Format(time.RFC3339),
		"data_points_checked", "multiple_exchanges")

	return highestPrice, nil
}

func (r *RedisAdapter) GetHighestPriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error) {
	slog.Debug("Getting highest price in range by exchange from cache",
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

	highestPrice := &prices[0]
	maxPrice := prices[0].Price

	for i := 1; i < len(prices); i++ {
		if prices[i].Price > maxPrice {
			maxPrice = prices[i].Price
			highestPrice = &prices[i]
		}
	}

	slog.Info("Found highest price in cache for exchange",
		"symbol", symbol,
		"exchange", exchange,
		"price", highestPrice.Price,
		"timestamp", time.UnixMilli(highestPrice.Timestamp).Format(time.RFC3339),
		"data_points_checked", len(prices))

	return highestPrice, nil
}

func getMaxPriceFromData(prices []domain.MarketData) float64 {
	if len(prices) == 0 {
		return 0
	}

	maxPrice := prices[0].Price
	for _, price := range prices[1:] {
		if price.Price > maxPrice {
			maxPrice = price.Price
		}
	}
	return maxPrice
}

func (r *RedisAdapter) GetLatestPricesCount(ctx context.Context, symbol, exchange string, count int) ([]domain.MarketData, error) {
	key := fmt.Sprintf("timeseries:%s:%s", symbol, exchange)

	results, err := r.client.ZRevRange(ctx, key, 0, int64(count-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest prices: %w", err)
	}

	var data []domain.MarketData
	for _, result := range results {

		var price float64
		if _, err := fmt.Sscanf(result, "%f", &price); err != nil {
			continue
		}

		scores, err := r.client.ZScore(ctx, key, result).Result()
		if err != nil {
			continue
		}

		timestamp := int64(scores)

		marketData := domain.MarketData{
			Symbol:    symbol,
			Price:     price,
			Timestamp: timestamp,
			Exchange:  exchange,
		}
		data = append(data, marketData)
	}

	return data, nil
}
