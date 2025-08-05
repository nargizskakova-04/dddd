// internal/core/service/prices/prices_with_period.go
package prices

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cryptomarket/internal/core/domain"
)

// GetHighestPriceWithPeriod returns the highest price for a symbol within a specified period
// Uses Redis for sub-minute periods and PostgreSQL for minute+ periods
func (s *PriceService) GetHighestPriceWithPeriod(ctx context.Context, symbol, periodStr string) (*domain.MarketData, error) {
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

	slog.Info("Getting highest price with period",
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
		result, err = s.getHighestPriceFromCache(ctx, validSymbol, allowedExchanges, startTime, endTime, duration)
		if err != nil {
			slog.Error("Failed to get highest price from cache",
				"symbol", validSymbol,
				"period", periodStr,
				"error", err)
			return nil, fmt.Errorf("failed to get highest price from cache: %w", err)
		}
	} else {
		// Use PostgreSQL for minute+ periods
		result, err = s.getHighestPriceFromDatabase(ctx, validSymbol, allowedExchanges, startTime, endTime, duration)
		if err != nil {
			slog.Error("Failed to get highest price from database",
				"symbol", validSymbol,
				"period", periodStr,
				"error", err)
			return nil, fmt.Errorf("failed to get highest price from database: %w", err)
		}
	}

	if result == nil {
		return nil, fmt.Errorf("no price data found for symbol %s in the last %s", validSymbol, periodStr)
	}

	slog.Info("Successfully retrieved highest price with period",
		"symbol", validSymbol,
		"period", periodStr,
		"price", result.Price,
		"exchange", result.Exchange,
		"timestamp", time.Unix(result.Timestamp, 0).Format(time.RFC3339),
		"data_source", map[bool]string{true: "Redis", false: "PostgreSQL"}[useCache])

	return result, nil
}

// GetHighestPriceByExchangeWithPeriod returns the highest price for a symbol from specific exchange within a period
func (s *PriceService) GetHighestPriceByExchangeWithPeriod(ctx context.Context, symbol, exchange, periodStr string) (*domain.MarketData, error) {
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

	slog.Info("Getting highest price by exchange with period",
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
		result, err = s.getHighestPriceByExchangeFromCache(ctx, validSymbol, validExchange, startTime, endTime, duration)
		if err != nil {
			slog.Error("Failed to get highest price by exchange from cache",
				"symbol", validSymbol,
				"exchange", validExchange,
				"period", periodStr,
				"error", err)
			return nil, fmt.Errorf("failed to get highest price from cache: %w", err)
		}
	} else {
		// Use PostgreSQL for minute+ periods
		result, err = s.getHighestPriceByExchangeFromDatabase(ctx, validSymbol, validExchange, startTime, endTime, duration)
		if err != nil {
			slog.Error("Failed to get highest price by exchange from database",
				"symbol", validSymbol,
				"exchange", validExchange,
				"period", periodStr,
				"error", err)
			return nil, fmt.Errorf("failed to get highest price from database: %w", err)
		}
	}

	if result == nil {
		return nil, fmt.Errorf("no price data found for symbol %s on exchange %s in the last %s", validSymbol, validExchange, periodStr)
	}

	slog.Info("Successfully retrieved highest price by exchange with period",
		"symbol", validSymbol,
		"exchange", validExchange,
		"period", periodStr,
		"price", result.Price,
		"timestamp", time.Unix(result.Timestamp, 0).Format(time.RFC3339),
		"data_source", map[bool]string{true: "Redis", false: "PostgreSQL"}[useCache])

	return result, nil
}

// Helper methods for cache operations

func (s *PriceService) getHighestPriceFromCache(ctx context.Context, symbol string, exchanges []string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("cache not available")
	}

	// Check if cache has the method we need
	type HighestPriceCache interface {
		GetHighestPriceInRange(ctx context.Context, symbol string, exchanges []string, from, to time.Time) (*domain.MarketData, error)
	}

	cacheWithHighest, ok := s.cache.(HighestPriceCache)
	if !ok {
		return nil, fmt.Errorf("cache does not support highest price range queries")
	}

	slog.Debug("Getting highest price from cache",
		"symbol", symbol,
		"exchanges", exchanges,
		"duration", duration)

	return cacheWithHighest.GetHighestPriceInRange(ctx, symbol, exchanges, startTime, endTime)
}

func (s *PriceService) getHighestPriceByExchangeFromCache(ctx context.Context, symbol, exchange string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.cache == nil {
		return nil, fmt.Errorf("cache not available")
	}

	// Check if cache has the method we need
	type HighestPriceByExchangeCache interface {
		GetHighestPriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error)
	}

	cacheWithHighest, ok := s.cache.(HighestPriceByExchangeCache)
	if !ok {
		return nil, fmt.Errorf("cache does not support highest price range queries by exchange")
	}

	slog.Debug("Getting highest price by exchange from cache",
		"symbol", symbol,
		"exchange", exchange,
		"duration", duration)

	return cacheWithHighest.GetHighestPriceInRangeByExchange(ctx, symbol, exchange, startTime, endTime)
}

// Helper methods for database operations

func (s *PriceService) getHighestPriceFromDatabase(ctx context.Context, symbol string, exchanges []string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	// Check if repository has the method we need
	type HighestPriceRepository interface {
		GetHighestPriceInRange(ctx context.Context, symbol string, allowedExchanges []string, from, to time.Time) (*domain.MarketData, error)
	}

	repoWithHighest, ok := s.pricesRepo.(HighestPriceRepository)
	if !ok {
		return nil, fmt.Errorf("repository does not support highest price range queries")
	}

	slog.Debug("Getting highest price from database",
		"symbol", symbol,
		"exchanges", exchanges,
		"duration", duration)

	return repoWithHighest.GetHighestPriceInRange(ctx, symbol, exchanges, startTime, endTime)
}

func (s *PriceService) getHighestPriceByExchangeFromDatabase(ctx context.Context, symbol, exchange string, startTime, endTime time.Time, duration time.Duration) (*domain.MarketData, error) {
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	// Check if repository has the method we need
	type HighestPriceByExchangeRepository interface {
		GetHighestPriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error)
	}

	repoWithHighest, ok := s.pricesRepo.(HighestPriceByExchangeRepository)
	if !ok {
		return nil, fmt.Errorf("repository does not support highest price range queries by exchange")
	}

	slog.Debug("Getting highest price by exchange from database",
		"symbol", symbol,
		"exchange", exchange,
		"duration", duration)

	return repoWithHighest.GetHighestPriceInRangeByExchange(ctx, symbol, exchange, startTime, endTime)
}

// GetPeriodInfo returns information about how a period will be processed (for debugging/API info)
func (s *PriceService) GetPeriodInfo(periodStr string) (map[string]interface{}, error) {
	durationInfo, err := GetDurationInfo(periodStr)
	if err != nil {
		return nil, err
	}

	info := map[string]interface{}{
		"period":            periodStr,
		"duration":          durationInfo.Duration.String(),
		"duration_seconds":  durationInfo.Duration.Seconds(),
		"use_cache":         durationInfo.UseCache,
		"data_source":       durationInfo.DataSource,
		"description":       durationInfo.Description,
		"start_time":        durationInfo.StartTime.Format(time.RFC3339),
		"end_time":          durationInfo.EndTime.Format(time.RFC3339),
		"current_mode":      s.getCurrentMode(),
		"allowed_exchanges": s.getAllowedExchanges(),
	}

	return info, nil
}
