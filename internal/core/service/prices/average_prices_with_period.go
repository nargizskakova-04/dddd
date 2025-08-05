// internal/core/service/prices/average_prices_with_period.go - FIXED VERSION
package prices

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cryptomarket/internal/core/domain"
)

// GetAveragePriceWithPeriod returns the average price for a symbol within a specified period
// Uses Redis for sub-minute periods and PostgreSQL for minute+ periods
func (s *PriceService) GetAveragePriceWithPeriod(ctx context.Context, symbol, periodStr string) (*domain.MarketData, error) {
	// Validate symbol
	validSymbol, err := s.validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	// Parse and validate duration
	duration, useCache, err := ParseDuration(periodStr)
	if err != nil {
		return nil, fmt.Errorf("invalid period: %w", err)
	}

	if err := ValidateDuration(duration); err != nil {
		return nil, fmt.Errorf("invalid period: %w", err)
	}

	// Get allowed exchanges for current mode
	allowedExchanges := s.getAllowedExchanges()
	if len(allowedExchanges) == 0 {
		return nil, fmt.Errorf("no exchanges available for current mode")
	}

	// Calculate time range
	startTime, endTime := GetTimeRange(duration)

	slog.Info("Getting average price with period",
		"symbol", validSymbol,
		"period", periodStr,
		"duration", duration,
		"use_cache", useCache,
		"allowed_exchanges", allowedExchanges,
		"start_time", startTime.Format(time.RFC3339),
		"end_time", endTime.Format(time.RFC3339),
		"current_mode", s.getCurrentMode())

	var result *domain.MarketData

	if useCache {
		// Use Redis for sub-minute periods
		result, err = s.getAveragePriceFromCache(ctx, validSymbol, allowedExchanges, startTime, endTime, duration)
		if err != nil {
			slog.Error("Failed to get average price from cache",
				"symbol", validSymbol,
				"period", periodStr,
				"error", err)
			return nil, fmt.Errorf("failed to get average price from cache: %w", err)
		}
	} else {
		// Use PostgreSQL for minute+ periods
		result, err = s.getAveragePriceFromDatabase(ctx, validSymbol, allowedExchanges, startTime, endTime, duration)
		if err != nil {
			slog.Error("Failed to get average price from database",
				"symbol", validSymbol,
				"period", periodStr,
				"error", err)
			return nil, fmt.Errorf("failed to get average price from database: %w", err)
		}
	}

	if result == nil {
		return nil, fmt.Errorf("no price data found for symbol %s in the last %s", validSymbol, periodStr)
	}

	slog.Info("Successfully retrieved average price with period",
		"symbol", validSymbol,
		"period", periodStr,
		"price", result.Price,
		"exchange", result.Exchange,
		"timestamp", time.Unix(result.Timestamp, 0).Format(time.RFC3339),
		"data_source", map[bool]string{true: "Redis", false: "PostgreSQL"}[useCache])

	return result, nil
}

// GetAveragePriceByExchangeWithPeriod returns the average price for a symbol from specific exchange within a period
func (s *PriceService) GetAveragePriceByExchangeWithPeriod(ctx context.Context, symbol, exchange, periodStr string) (*domain.MarketData, error) {
	// Validate symbol and exchange
	validSymbol, err := s.validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	validExchange, err := s.validateExchange(exchange)
	if err != nil {
		return nil, err
	}

	// Check if exchange is allowed in current mode
	if !s.isExchangeAllowedInCurrentMode(validExchange) {
		currentMode := s.getCurrentMode()
		return nil, fmt.Errorf("exchange %s is not available in %s mode", exchange, currentMode)
	}

	// Parse and validate duration
	duration, useCache, err := ParseDuration(periodStr)
	if err != nil {
		return nil, fmt.Errorf("invalid period: %w", err)
	}

	if err := ValidateDuration(duration); err != nil {
		return nil, fmt.Errorf("invalid period: %w", err)
	}

	// Calculate time range
	startTime, endTime := GetTimeRange(duration)

	slog.Info("Getting average price by exchange with period",
		"symbol", validSymbol,
		"exchange", validExchange,
		"period", periodStr,
		"duration", duration,
		"use_cache", useCache,
		"start_time", startTime.Format(time.RFC3339),
		"end_time", endTime.Format(time.RFC3339),
		"current_mode", s.getCurrentMode())

	var result *domain.MarketData

	if useCache {
		// Use Redis for sub-minute periods
		result, err = s.getAveragePriceByExchangeFromCache(ctx, validSymbol, validExchange, startTime, endTime, duration)
		if err != nil {
			slog.Error("Failed to get average price by exchange from cache",
				"symbol", validSymbol,
				"exchange", validExchange,
				"period", periodStr,
				"error", err)
			return nil, fmt.Errorf("failed to get average price from cache: %w", err)
		}
	} else {
		// Use PostgreSQL for minute+ periods
		result, err = s.getAveragePriceByExchangeFromDatabase(ctx, validSymbol, validExchange, startTime, endTime, duration)
		if err != nil {
			slog.Error("Failed to get average price by exchange from database",
				"symbol", validSymbol,
				"exchange", validExchange,
				"period", periodStr,
				"error", err)
			return nil, fmt.Errorf("failed to get average price from database: %w", err)
		}
	}

	if result == nil {
		return nil, fmt.Errorf("no price data found for symbol %s on exchange %s in the last %s", validSymbol, validExchange, periodStr)
	}

	slog.Info("Successfully retrieved average price by exchange with period",
		"symbol", validSymbol,
		"exchange", validExchange,
		"period", periodStr,
		"price", result.Price,
		"timestamp", time.Unix(result.Timestamp, 0).Format(time.RFC3339),
		"data_source", map[bool]string{true: "Redis", false: "PostgreSQL"}[useCache])

	return result, nil
}

// FIXED: Helper methods for cache operations - now calculate actual average

func (s *PriceService) getAveragePriceFromCache(ctx context.Context, symbol string, exchanges []string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("cache not available")
	}

	slog.Debug("Getting average price from cache",
		"symbol", symbol,
		"exchanges", exchanges,
		"duration", duration)

	// Get all price data from all exchanges in the time range
	var allPrices []domain.MarketData

	for _, exchange := range exchanges {
		prices, err := s.cache.GetPricesInRangeByExchange(ctx, symbol, exchange, startTime, endTime)
		if err != nil {
			slog.Debug("No prices found for exchange in cache",
				"exchange", exchange,
				"symbol", symbol,
				"error", err)
			continue // Skip this exchange if no data
		}

		if len(prices) > 0 {
			allPrices = append(allPrices, prices...)
		}
	}

	if len(allPrices) == 0 {
		return nil, fmt.Errorf("no price data found in cache for any exchange")
	}

	// FIXED: Calculate actual average price from all the raw prices
	var sum float64
	var latestTimestamp int64
	var latestExchange string

	for _, price := range allPrices {
		sum += price.Price
		if price.Timestamp > latestTimestamp {
			latestTimestamp = price.Timestamp
			latestExchange = price.Exchange
		}
	}

	averagePrice := sum / float64(len(allPrices))

	result := &domain.MarketData{
		Symbol:    symbol,
		Price:     averagePrice, // FIXED: Use calculated average
		Timestamp: latestTimestamp,
		Exchange:  "multiple", // Indicate this is from multiple exchanges
	}

	if len(exchanges) == 1 {
		result.Exchange = latestExchange // Use actual exchange name if only one
	}

	slog.Debug("Calculated average price from cache",
		"symbol", symbol,
		"average_price", averagePrice,
		"data_points", len(allPrices),
		"exchanges_checked", len(exchanges))

	return result, nil
}

func (s *PriceService) getAveragePriceByExchangeFromCache(ctx context.Context, symbol, exchange string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("cache not available")
	}

	slog.Debug("Getting average price by exchange from cache",
		"symbol", symbol,
		"exchange", exchange,
		"duration", duration)

	// Get price data from specific exchange
	prices, err := s.cache.GetPricesInRangeByExchange(ctx, symbol, exchange, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get prices from cache: %w", err)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no price data found in cache for exchange %s", exchange)
	}

	// FIXED: Calculate actual average price from all the raw prices
	var sum float64
	var latestTimestamp int64

	for _, price := range prices {
		sum += price.Price
		if price.Timestamp > latestTimestamp {
			latestTimestamp = price.Timestamp
		}
	}

	averagePrice := sum / float64(len(prices))

	result := &domain.MarketData{
		Symbol:    symbol,
		Price:     averagePrice, // FIXED: Use calculated average
		Timestamp: latestTimestamp,
		Exchange:  exchange,
	}

	slog.Debug("Calculated average price by exchange from cache",
		"symbol", symbol,
		"exchange", exchange,
		"average_price", averagePrice,
		"data_points", len(prices))

	return result, nil
}

// Helper methods for database operations (these should work correctly with the fixed database methods)

func (s *PriceService) getAveragePriceFromDatabase(ctx context.Context, symbol string, exchanges []string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	// Check if repository has the method we need
	type AveragePriceRepository interface {
		GetAveragePriceInRange(ctx context.Context, symbol string, allowedExchanges []string, from, to time.Time) (*domain.MarketData, error)
	}

	repoWithAverage, ok := s.pricesRepo.(AveragePriceRepository)
	if !ok {
		return nil, fmt.Errorf("repository does not support average price range queries")
	}

	slog.Debug("Getting average price from database",
		"symbol", symbol,
		"exchanges", exchanges,
		"duration", duration)

	return repoWithAverage.GetAveragePriceInRange(ctx, symbol, exchanges, startTime, endTime)
}

func (s *PriceService) getAveragePriceByExchangeFromDatabase(ctx context.Context, symbol, exchange string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	// Check if repository has the method we need
	type AveragePriceByExchangeRepository interface {
		GetAveragePriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error)
	}

	repoWithAverage, ok := s.pricesRepo.(AveragePriceByExchangeRepository)
	if !ok {
		return nil, fmt.Errorf("repository does not support average price range queries by exchange")
	}

	slog.Debug("Getting average price by exchange from database",
		"symbol", symbol,
		"exchange", exchange,
		"duration", duration)

	return repoWithAverage.GetAveragePriceInRangeByExchange(ctx, symbol, exchange, startTime, endTime)
}
