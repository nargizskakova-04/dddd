package prices

import (
	"context"
	"database/sql"
	"fmt"

	"crypto/internal/core/domain"
	"crypto/internal/core/port"
)

type PriceService struct {
	cache port.Cache
	db    *sql.DB
}

// NewPriceService creates a new price service with proper interface dependency
func NewPriceService(cache port.Cache, db *sql.DB) port.PriceService {
	return &PriceService{
		cache: cache,
		db:    db,
	}
}

func (s *PriceService) GetLatestPrice(ctx context.Context, symbol string) (*domain.MarketData, error) {
	// If cache is available, try to get from cache first
	if s.cache != nil {
		data, err := s.cache.GetLatestPrice(ctx, symbol)
		if err == nil && data != nil {
			return data, nil
		}
		// If cache fails or no data, continue to fallback
	}

	// TODO: Fallback to PostgreSQL if cache is unavailable
	// For now, return error if cache is not available
	if s.cache == nil {
		return nil, fmt.Errorf("no cache available and PostgreSQL fallback not implemented")
	}

	return nil, fmt.Errorf("no price data found for symbol %s", symbol)
}

func (s *PriceService) GetLatestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error) {
	// If cache is available, try to get from cache first
	if s.cache != nil {
		data, err := s.cache.GetLatestPriceByExchange(ctx, symbol, exchange)
		if err == nil && data != nil {
			return data, nil
		}
		// If cache fails or no data, continue to fallback
	}

	// TODO: Fallback to PostgreSQL if cache is unavailable
	// For now, return error if cache is not available
	if s.cache == nil {
		return nil, fmt.Errorf("no cache available and PostgreSQL fallback not implemented")
	}

	return nil, fmt.Errorf("no price data found for symbol %s on exchange %s", symbol, exchange)
}
