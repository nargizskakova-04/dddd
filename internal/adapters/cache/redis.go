package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"crypto/internal/core/domain"
	"crypto/internal/core/port"

	"github.com/redis/go-redis/v9"
)

type RedisAdapter struct {
	client *redis.Client
}

func NewRedisAdapter(client *redis.Client) port.Cache {
	return &RedisAdapter{
		client: client,
	}
}

// SetPrice stores price data with timestamp
func (r *RedisAdapter) SetPrice(ctx context.Context, key string, data domain.MarketData) error {
	// Store latest price for quick access
	latestKey := fmt.Sprintf("latest:%s:%s", data.Symbol, data.Exchange)
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal market data: %w", err)
	}

	// Set latest price with 2 minute expiration (to ensure cleanup)
	if err := r.client.Set(ctx, latestKey, dataBytes, 2*time.Minute).Err(); err != nil {
		return fmt.Errorf("failed to set latest price: %w", err)
	}

	// Store in time-series sorted set for range queries
	timeSeriesKey := fmt.Sprintf("timeseries:%s:%s", data.Symbol, data.Exchange)
	score := float64(data.Timestamp)
	member := fmt.Sprintf("%f", data.Price)

	// Add to sorted set with score as timestamp
	if err := r.client.ZAdd(ctx, timeSeriesKey, redis.Z{
		Score:  score,
		Member: member,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to time series: %w", err)
	}

	// Set expiration for time series (2 minutes to ensure cleanup)
	r.client.Expire(ctx, timeSeriesKey, 2*time.Minute)

	return nil
}

// GetLatestPrice retrieves the latest price for a symbol across all exchanges
func (r *RedisAdapter) GetLatestPrice(ctx context.Context, symbol string) (*domain.MarketData, error) {
	pattern := fmt.Sprintf("latest:%s:*", symbol)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no price data found for symbol %s", symbol)
	}

	// Get the most recent price
	var latestData *domain.MarketData
	var latestTimestamp int64

	for _, key := range keys {
		dataStr, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var data domain.MarketData
		if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
			continue
		}

		if data.Timestamp > latestTimestamp {
			latestTimestamp = data.Timestamp
			latestData = &data
		}
	}

	if latestData == nil {
		return nil, fmt.Errorf("no valid price data found for symbol %s", symbol)
	}

	return latestData, nil
}

// GetLatestPriceByExchange retrieves the latest price for a symbol from specific exchange
func (r *RedisAdapter) GetLatestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error) {
	key := fmt.Sprintf("latest:%s:%s", symbol, exchange)

	dataStr, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("no price data found for %s on %s", symbol, exchange)
		}
		return nil, fmt.Errorf("failed to get price data: %w", err)
	}

	var data domain.MarketData
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal price data: %w", err)
	}

	return &data, nil
}

// GetPricesInRange retrieves all prices for a symbol within time range across all exchanges
func (r *RedisAdapter) GetPricesInRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.MarketData, error) {
	// Get all exchanges for this symbol
	pattern := fmt.Sprintf("timeseries:%s:*", symbol)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get timeseries keys: %w", err)
	}

	var allData []domain.MarketData
	fromScore := float64(from.Unix())
	toScore := float64(to.Unix())

	for _, key := range keys {
		// Extract exchange name from key
		// timeseries:SYMBOL:EXCHANGE -> EXCHANGE
		parts := parseTimeSeriesKey(key)
		if len(parts) < 3 {
			continue
		}
		exchange := parts[2]

		// Get prices in range
		results, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
			Min: strconv.FormatFloat(fromScore, 'f', -1, 64),
			Max: strconv.FormatFloat(toScore, 'f', -1, 64),
		}).Result()
		if err != nil {
			continue
		}

		// Convert to MarketData
		for _, result := range results {
			price, err := strconv.ParseFloat(result, 64)
			if err != nil {
				continue
			}

			// Get timestamp from score
			scores, err := r.client.ZScore(ctx, key, result).Result()
			if err != nil {
				continue
			}

			data := domain.MarketData{
				Symbol:    symbol,
				Price:     price,
				Timestamp: int64(scores),
				Exchange:  exchange,
			}
			allData = append(allData, data)
		}
	}

	return allData, nil
}

// GetPricesInRangeByExchange retrieves prices for a symbol from specific exchange within time range
func (r *RedisAdapter) GetPricesInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) ([]domain.MarketData, error) {
	key := fmt.Sprintf("timeseries:%s:%s", symbol, exchange)
	fromScore := float64(from.Unix())
	toScore := float64(to.Unix())

	results, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: strconv.FormatFloat(fromScore, 'f', -1, 64),
		Max: strconv.FormatFloat(toScore, 'f', -1, 64),
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get prices in range: %w", err)
	}

	var data []domain.MarketData
	for _, result := range results {
		price, err := strconv.ParseFloat(result, 64)
		if err != nil {
			continue
		}

		// Get timestamp from score
		scores, err := r.client.ZScore(ctx, key, result).Result()
		if err != nil {
			continue
		}

		marketData := domain.MarketData{
			Symbol:    symbol,
			Price:     price,
			Timestamp: int64(scores),
			Exchange:  exchange,
		}
		data = append(data, marketData)
	}

	return data, nil
}

// CleanupOldData removes data older than specified duration
func (r *RedisAdapter) CleanupOldData(ctx context.Context, olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)
	cutoffScore := float64(cutoffTime.Unix())

	// Clean up time series data
	timeSeriesPattern := "timeseries:*"
	keys, err := r.client.Keys(ctx, timeSeriesPattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get timeseries keys for cleanup: %w", err)
	}

	for _, key := range keys {
		// Remove old entries from sorted set
		_, err := r.client.ZRemRangeByScore(ctx, key, "0", strconv.FormatFloat(cutoffScore, 'f', -1, 64)).Result()
		if err != nil {
			continue // Continue with other keys
		}
	}

	// Clean up latest price data (handled by TTL, but we can also check manually)
	latestPattern := "latest:*"
	latestKeys, err := r.client.Keys(ctx, latestPattern).Result()
	if err == nil {
		for _, key := range latestKeys {
			// Check if the data is too old
			dataStr, err := r.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var data domain.MarketData
			if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
				continue
			}

			if time.Unix(data.Timestamp, 0).Before(cutoffTime) {
				r.client.Del(ctx, key)
			}
		}
	}

	return nil
}

// Ping checks Redis connection health
func (r *RedisAdapter) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// parseTimeSeriesKey parses a timeseries key to extract components
func parseTimeSeriesKey(key string) []string {
	result := make([]string, 0)
	current := ""
	for _, char := range key {
		if char == ':' {
			result = append(result, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
