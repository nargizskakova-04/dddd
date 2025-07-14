package prices

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"cryptomarket/internal/core/domain"
	"cryptomarket/internal/core/port"
)

// Supported symbols and exchanges (simple maps)
var supportedSymbols = map[string]bool{
	"BTCUSDT":  true,
	"DOGEUSDT": true,
	"TONUSDT":  true,
	"SOLUSDT":  true,
	"ETHUSDT":  true,
}

var supportedExchanges = map[string]bool{
	"exchange1":      true,
	"exchange2":      true,
	"exchange3":      true,
	"test-exchange1": true,
	"test-exchange2": true,
	"test-exchange3": true,
}

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
	// Validate symbol
	validSymbol, err := s.validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	// If cache is available, try to get from cache first
	if s.cache != nil {
		data, err := s.cache.GetLatestPrice(ctx, validSymbol)
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
	// Validate symbol and exchange
	validSymbol, err := s.validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	validExchange, err := s.validateExchange(exchange)
	if err != nil {
		return nil, err
	}

	// If cache is available, try to get from cache first
	if s.cache != nil {
		data, err := s.cache.GetLatestPriceByExchange(ctx, validSymbol, validExchange)
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

// Simple validation functions
func (s *PriceService) validateSymbol(symbol string) (string, error) {
	if symbol == "" {
		return "", fmt.Errorf("symbol cannot be empty")
	}

	// Normalize to uppercase
	normalized := strings.ToUpper(strings.TrimSpace(symbol))

	// Check if supported
	if !supportedSymbols[normalized] {
		return "", fmt.Errorf("unsupported symbol: %s", symbol)
	}

	return normalized, nil
}

func (s *PriceService) validateExchange(exchange string) (string, error) {
	if exchange == "" {
		return "", fmt.Errorf("exchange cannot be empty")
	}

	// Normalize to lowercase
	normalized := strings.ToLower(strings.TrimSpace(exchange))

	// Check if supported
	if !supportedExchanges[normalized] {
		return "", fmt.Errorf("unsupported exchange: %s", exchange)
	}

	return normalized, nil
}
