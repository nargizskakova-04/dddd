package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"cryptomarket/internal/core/domain"
	"cryptomarket/internal/core/port"

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

func (r *RedisAdapter) SetPrice(ctx context.Context, key string, data domain.MarketData) error {
	latestKey := fmt.Sprintf("latest:%s:%s", data.Symbol, data.Exchange)
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal market data: %w", err)
	}

	if err := r.client.Set(ctx, latestKey, dataBytes, 2*time.Minute).Err(); err != nil {
		return fmt.Errorf("failed to set latest price: %w", err)
	}

	timeSeriesKey := fmt.Sprintf("timeseries:%s:%s", data.Symbol, data.Exchange)

	score := float64(data.Timestamp)
	member := fmt.Sprintf("%f", data.Price)

	if err := r.client.ZAdd(ctx, timeSeriesKey, redis.Z{
		Score:  score,
		Member: member,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to time series: %w", err)
	}

	r.client.Expire(ctx, timeSeriesKey, 2*time.Minute)

	return nil
}

func (r *RedisAdapter) GetLatestPrice(ctx context.Context, symbol string) (*domain.MarketData, error) {
	pattern := fmt.Sprintf("latest:%s:*", symbol)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no price data found for symbol %s", symbol)
	}

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

func (r *RedisAdapter) GetLatestPriceFromExchanges(ctx context.Context, symbol string, exchanges []string) (*domain.MarketData, error) {
	if len(exchanges) == 0 {
		return nil, fmt.Errorf("no exchanges specified")
	}

	var latestData *domain.MarketData
	var latestTimestamp int64

	for _, exchange := range exchanges {
		key := fmt.Sprintf("latest:%s:%s", symbol, exchange)

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
		return nil, fmt.Errorf("no price data found for symbol %s from exchanges %v", symbol, exchanges)
	}

	return latestData, nil
}

func (r *RedisAdapter) GetPricesInRange(ctx context.Context, symbol string, from, to time.Time) ([]domain.MarketData, error) {
	pattern := fmt.Sprintf("timeseries:%s:*", symbol)
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get timeseries keys: %w", err)
	}

	var allData []domain.MarketData

	fromScore := float64(from.UnixMilli())
	toScore := float64(to.UnixMilli())

	for _, key := range keys {
		parts := parseTimeSeriesKey(key)
		if len(parts) < 3 {
			continue
		}
		exchange := parts[2]

		results, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
			Min: strconv.FormatFloat(fromScore, 'f', -1, 64),
			Max: strconv.FormatFloat(toScore, 'f', -1, 64),
		}).Result()
		if err != nil {
			continue
		}

		for _, result := range results {
			price, err := strconv.ParseFloat(result, 64)
			if err != nil {
				continue
			}

			scores, err := r.client.ZScore(ctx, key, result).Result()
			if err != nil {
				continue
			}

			timestamp := int64(scores)

			data := domain.MarketData{
				Symbol:    symbol,
				Price:     price,
				Timestamp: timestamp,
				Exchange:  exchange,
			}
			allData = append(allData, data)
		}
	}

	return allData, nil
}

func (r *RedisAdapter) GetPricesInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) ([]domain.MarketData, error) {
	key := fmt.Sprintf("timeseries:%s:%s", symbol, exchange)

	fromScore := float64(from.UnixMilli())
	toScore := float64(to.UnixMilli())

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
func (r *RedisAdapter) GetPricesInRangeFromExchanges(ctx context.Context, symbol string, exchanges []string, from, to time.Time) ([]domain.MarketData, error) {
	if len(exchanges) == 0 {
		return nil, fmt.Errorf("no exchanges specified")
	}

	var allData []domain.MarketData

	fromScore := float64(from.UnixMilli())
	toScore := float64(to.UnixMilli())

	for _, exchange := range exchanges {
		key := fmt.Sprintf("timeseries:%s:%s", symbol, exchange)

		results, err := r.client.ZRangeByScore(ctx, key, &redis.ZRangeBy{
			Min: strconv.FormatFloat(fromScore, 'f', -1, 64),
			Max: strconv.FormatFloat(toScore, 'f', -1, 64),
		}).Result()
		if err != nil {
			continue
		}

		for _, result := range results {
			price, err := strconv.ParseFloat(result, 64)
			if err != nil {
				continue
			}

			scores, err := r.client.ZScore(ctx, key, result).Result()
			if err != nil {
				continue
			}

			timestamp := int64(scores)

			data := domain.MarketData{
				Symbol:    symbol,
				Price:     price,
				Timestamp: timestamp,
				Exchange:  exchange,
			}
			allData = append(allData, data)
		}
	}

	return allData, nil
}

func (r *RedisAdapter) CleanupOldData(ctx context.Context, olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)

	cutoffScore := float64(cutoffTime.UnixMilli())

	timeSeriesPattern := "timeseries:*"
	keys, err := r.client.Keys(ctx, timeSeriesPattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get timeseries keys for cleanup: %w", err)
	}

	for _, key := range keys {

		_, err := r.client.ZRemRangeByScore(ctx, key, "0", strconv.FormatFloat(cutoffScore, 'f', -1, 64)).Result()
		if err != nil {
			continue
		}
	}

	latestPattern := "latest:*"
	latestKeys, err := r.client.Keys(ctx, latestPattern).Result()
	if err == nil {
		for _, key := range latestKeys {

			dataStr, err := r.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var data domain.MarketData
			if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
				continue
			}

			dataTime := time.UnixMilli(data.Timestamp)

			if dataTime.Before(cutoffTime) {
				r.client.Del(ctx, key)
			}
		}
	}

	return nil
}

func (r *RedisAdapter) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

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
