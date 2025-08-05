// internal/core/service/prices/prices_with_period.go
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

// Helper methods for cache operations

func (s *PriceService) getAveragePriceFromCache(ctx context.Context, symbol string, exchanges []string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("cache not available")
	}

	// Check if cache has the method we need
	type AveragePriceCache interface {
		GetAveragePriceInRange(ctx context.Context, symbol string, exchanges []string, from, to time.Time) (*domain.MarketData, error)
	}

	cacheWithAverage, ok := s.cache.(AveragePriceCache)
	if !ok {
		return nil, fmt.Errorf("cache does not support average price range queries")
	}

	slog.Debug("Getting average price from cache",
		"symbol", symbol,
		"exchanges", exchanges,
		"duration", duration)

	return cacheWithAverage.GetAveragePriceInRange(ctx, symbol, exchanges, startTime, endTime)
}

func (s *PriceService) getAveragePriceByExchangeFromCache(ctx context.Context, symbol, exchange string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("cache not available")
	}

	// Check if cache has the method we need
	type AveragePriceByExchangeCache interface {
		GetAveragePriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error)
	}

	cacheWithAverage, ok := s.cache.(AveragePriceByExchangeCache)
	if !ok {
		return nil, fmt.Errorf("cache does not support average price range queries by exchange")
	}

	slog.Debug("Getting average price by exchange from cache",
		"symbol", symbol,
		"exchange", exchange,
		"duration", duration)

	return cacheWithAverage.GetAveragePriceInRangeByExchange(ctx, symbol, exchange, startTime, endTime)
}

// Helper methods for database operations

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
