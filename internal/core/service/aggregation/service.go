// internal/core/service/aggregation/service.go - FIXED VERSION
package aggregation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cryptomarket/internal/core/domain"
	"cryptomarket/internal/core/port"
)

// AggregationService handles periodic aggregation of price data from Redis to PostgreSQL
type AggregationService struct {
	cache      port.Cache
	repository PricesRepository
	symbols    []string
	exchanges  []string
}

// PricesRepository interface for PostgreSQL operations
type PricesRepository interface {
	InsertAggregatedPrice(ctx context.Context, price domain.Prices) error
	InsertAggregatedPrices(ctx context.Context, prices []domain.Prices) error
	GetLatestAggregationTime(ctx context.Context) (time.Time, error)
	HealthCheck(ctx context.Context) error
}

// NewAggregationService creates a new aggregation service
func NewAggregationService(cache port.Cache, repository PricesRepository) *AggregationService {
	symbols := []string{"BTCUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT", "ETHUSDT"}
	exchanges := []string{"exchange1", "exchange2", "exchange3", "test-exchange1", "test-exchange2", "test-exchange3"}

	return &AggregationService{
		cache:      cache,
		repository: repository,
		symbols:    symbols,
		exchanges:  exchanges,
	}
}

func (s *AggregationService) AggregateData(ctx context.Context, aggregationTime time.Time) error {
	slog.Info("Starting data aggregation", "aggregation_time", aggregationTime)

	endTime := aggregationTime
	startTime := endTime.Add(-60 * time.Second)

	slog.Info("Aggregation time window",
		"start", startTime.Format(time.RFC3339),
		"end", endTime.Format(time.RFC3339),
		"start_unix", startTime.Unix(),
		"end_unix", endTime.Unix(),
		"start_unix_ms", startTime.UnixMilli(),
		"end_unix_ms", endTime.UnixMilli())

	var aggregatedPrices []domain.Prices
	aggregationCount := 0

	for _, symbol := range s.symbols {
		for _, exchange := range s.exchanges {
			aggregatedPrice, err := s.aggregateForSymbolExchange(ctx, symbol, exchange, startTime, endTime, aggregationTime)
			if err != nil {
				slog.Error("Failed to aggregate data for symbol/exchange",
					"symbol", symbol,
					"exchange", exchange,
					"error", err)
				continue
			}

			if aggregatedPrice != nil {
				aggregatedPrices = append(aggregatedPrices, *aggregatedPrice)
				aggregationCount++
			}
		}
	}

	if len(aggregatedPrices) == 0 {
		slog.Warn("No data to aggregate",
			"time_window", fmt.Sprintf("%s to %s", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339)),
			"start_unix_ms", startTime.UnixMilli(),
			"end_unix_ms", endTime.UnixMilli())
		return nil
	}

	if err := s.repository.InsertAggregatedPrices(ctx, aggregatedPrices); err != nil {
		return fmt.Errorf("failed to insert aggregated prices: %w", err)
	}

	slog.Info("Data aggregation completed successfully",
		"aggregated_records", aggregationCount,
		"inserted_records", len(aggregatedPrices),
		"aggregation_time", aggregationTime)

	return nil
}

func (s *AggregationService) aggregateForSymbolExchange(ctx context.Context, symbol, exchange string, startTime, endTime, aggregationTime time.Time) (*domain.Prices, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("cache is not available")
	}

	slog.Debug("Getting prices for symbol/exchange",
		"symbol", symbol,
		"exchange", exchange,
		"start_time", startTime.Format(time.RFC3339),
		"end_time", endTime.Format(time.RFC3339))

	priceData, err := s.cache.GetPricesInRangeByExchange(ctx, symbol, exchange, startTime, endTime)
	if err != nil {
		slog.Debug("No price data found in cache",
			"symbol", symbol,
			"exchange", exchange,
			"error", err)
		return nil, nil
	}

	if len(priceData) == 0 {
		slog.Debug("Empty price data for symbol/exchange",
			"symbol", symbol,
			"exchange", exchange)
		return nil, nil
	}

	aggregatedPrice := s.calculateAggregations(priceData, symbol, exchange, aggregationTime)

	slog.Info("Aggregated price data",
		"symbol", symbol,
		"exchange", exchange,
		"data_points", len(priceData),
		"avg_price", aggregatedPrice.AveragePrice,
		"min_price", aggregatedPrice.MinPrice,
		"max_price", aggregatedPrice.MaxPrice,
		"first_timestamp", priceData[0].Timestamp,
		"last_timestamp", priceData[len(priceData)-1].Timestamp)

	return aggregatedPrice, nil
}
func (s *AggregationService) calculateAggregations(data []domain.MarketData, symbol, exchange string, aggregationTime time.Time) *domain.Prices {
	if len(data) == 0 {
		return nil
	}

	var sum float64
	minPrice := data[0].Price
	maxPrice := data[0].Price

	for _, price := range data {
		sum += price.Price
		if price.Price < minPrice {
			minPrice = price.Price
		}
		if price.Price > maxPrice {
			maxPrice = price.Price
		}
	}

	averagePrice := sum / float64(len(data))

	return &domain.Prices{
		PairName:     symbol,
		Exchange:     exchange,
		Timestamp:    aggregationTime.UnixMilli(),
		AveragePrice: averagePrice,
		MinPrice:     minPrice,
		MaxPrice:     maxPrice,
	}
}

func (s *AggregationService) PerformPeriodicAggregation(ctx context.Context) {
	slog.Info("Starting periodic aggregation routine")

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	currentTime := time.Now().Truncate(time.Minute)
	if err := s.AggregateData(ctx, currentTime); err != nil {
		slog.Error("Initial aggregation failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("Periodic aggregation stopped")
			return

		case tickTime := <-ticker.C:
			aggregationTime := tickTime.Truncate(time.Minute)

			slog.Debug("Performing scheduled aggregation", "time", aggregationTime)

			if err := s.AggregateData(ctx, aggregationTime); err != nil {
				slog.Error("Periodic aggregation failed", "time", aggregationTime, "error", err)
			}
		}
	}
}

func (s *AggregationService) GetHealthStatus(ctx context.Context) map[string]interface{} {
	status := map[string]interface{}{
		"cache_available":      s.cache != nil,
		"repository_available": s.repository != nil,
		"symbols_count":        len(s.symbols),
		"exchanges_count":      len(s.exchanges),
	}

	if s.cache != nil {
		if err := s.cache.Ping(ctx); err != nil {
			status["cache_healthy"] = false
			status["cache_error"] = err.Error()
		} else {
			status["cache_healthy"] = true
		}
	}

	if s.repository != nil {
		if err := s.repository.HealthCheck(ctx); err != nil {
			status["repository_healthy"] = false
			status["repository_error"] = err.Error()
		} else {
			status["repository_healthy"] = true
		}
	}

	return status
}

func (s *AggregationService) TriggerManualAggregation(ctx context.Context, aggregationTime time.Time) error {
	slog.Info("Triggering manual aggregation", "time", aggregationTime)

	if err := s.AggregateData(ctx, aggregationTime); err != nil {
		return fmt.Errorf("manual aggregation failed: %w", err)
	}

	slog.Info("Manual aggregation completed successfully", "time", aggregationTime)
	return nil
}
