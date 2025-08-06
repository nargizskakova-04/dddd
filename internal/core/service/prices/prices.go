// internal/core/service/prices/prices.go
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
	cache           port.Cache
	db              *sql.DB
	exchangeService port.ExchangeService // NEW: Added to get current mode
	pricesRepo      port.PriceRepository
}

// NewPriceService creates a new price service with cache, database, and exchange service dependencies
func NewPriceService(cache port.Cache, db *sql.DB, exchangeService port.ExchangeService, pricesRepo port.PriceRepository) port.PriceService {
	return &PriceService{
		cache:           cache,
		db:              db,
		exchangeService: exchangeService,
		pricesRepo:      pricesRepo,
	}
}

// GetLatestPrice now filters by current mode
func (s *PriceService) GetLatestPrice(ctx context.Context, symbol string) (*domain.MarketData, error) {
	// Validate symbol
	validSymbol, err := s.validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	// If cache is available, try to get from cache with mode filtering
	if s.cache != nil {
		// Get allowed exchanges for current mode
		allowedExchanges := s.getAllowedExchanges()

		if len(allowedExchanges) == 0 {
			return nil, fmt.Errorf("no exchanges available for current mode")
		}

		// Use the new filtering method
		data, err := s.cache.GetLatestPriceFromExchanges(ctx, validSymbol, allowedExchanges)
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

	return nil, fmt.Errorf("no price data found for symbol %s in current mode", symbol)
}

// GetLatestPriceByExchange validates exchange is allowed in current mode
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

	// Check if exchange is allowed in current mode
	if !s.isExchangeAllowedInCurrentMode(validExchange) {
		currentMode := s.getCurrentMode()
		return nil, fmt.Errorf("exchange %s is not available in %s mode", exchange, currentMode)
	}

	// If cache is available, try to get from cache
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

// Helper methods for mode management

// getAllowedExchanges returns the list of exchanges allowed in the current mode
func (s *PriceService) getAllowedExchanges() []string {
	if s.exchangeService == nil {
		// Fallback to live mode exchanges if no exchange service
		return []string{"exchange1", "exchange2", "exchange3"}
	}

	// Use interface method to get exchanges for current mode
	// We need to add this method to the ExchangeService interface
	if exchSvc, ok := s.exchangeService.(interface{ GetModeExchanges() []string }); ok {
		return exchSvc.GetModeExchanges()
	}

	// Fallback to live mode
	return []string{"exchange1", "exchange2", "exchange3"}
}

// getCurrentMode returns the current mode
func (s *PriceService) getCurrentMode() string {
	if s.exchangeService == nil {
		return "live" // Default fallback
	}
	return s.exchangeService.GetCurrentMode()
}

// isExchangeAllowedInCurrentMode checks if an exchange is allowed in the current mode
func (s *PriceService) isExchangeAllowedInCurrentMode(exchange string) bool {
	allowedExchanges := s.getAllowedExchanges()

	for _, allowed := range allowedExchanges {
		if allowed == exchange {
			return true
		}
	}

	return false
}

// Simple validation functions (unchanged)
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

// GetCurrentModeInfo returns information about the current mode and available exchanges
func (s *PriceService) GetCurrentModeInfo() map[string]interface{} {
	return map[string]interface{}{
		"current_mode":      s.getCurrentMode(),
		"allowed_exchanges": s.getAllowedExchanges(),
		"total_exchanges":   6,
		"live_exchanges":    []string{"exchange1", "exchange2", "exchange3"},
		"test_exchanges":    []string{"test-exchange1", "test-exchange2", "test-exchange3"},
	}
}

// GetHighestPrice returns the highest price for a symbol from last 30 records, filtered by current mode
func (s *PriceService) GetHighestPrice(ctx context.Context, symbol string) (*domain.MarketData, error) {
	// Validate symbol
	validSymbol, err := s.validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	// Get allowed exchanges for current mode
	allowedExchanges := s.getAllowedExchanges()
	if len(allowedExchanges) == 0 {
		return nil, fmt.Errorf("no exchanges available for current mode")
	}

	// Get from database
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	data, err := s.pricesRepo.GetHighestPriceFromLatestRecords(ctx, validSymbol, allowedExchanges)
	if err != nil {
		return nil, fmt.Errorf("failed to get highest price from database: %w", err)
	}

	if data == nil {
		return nil, fmt.Errorf("no price data found for symbol %s in current mode", validSymbol)
	}

	return data, nil
}

// GetHighestPriceByExchange returns the highest price for a symbol and specific exchange from last 30 records
func (s *PriceService) GetHighestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error) {
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

	// Get from database
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	data, err := s.pricesRepo.GetHighestPriceByExchangeFromLatestRecord(ctx, validSymbol, validExchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get highest price by exchange from database: %w", err)
	}

	if data == nil {
		return nil, fmt.Errorf("no price data found for symbol %s on exchange %s", validSymbol, validExchange)
	}

	return data, nil
}

// GetHighestPrice returns the highest price for a symbol from last 30 records, filtered by current mode
func (s *PriceService) GetLowestPrice(ctx context.Context, symbol string) (*domain.MarketData, error) {
	// Validate symbol
	validSymbol, err := s.validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	// Get allowed exchanges for current mode
	allowedExchanges := s.getAllowedExchanges()
	if len(allowedExchanges) == 0 {
		return nil, fmt.Errorf("no exchanges available for current mode")
	}

	// Get from database
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	data, err := s.pricesRepo.GetLowestPriceFromLatestRecords(ctx, validSymbol, allowedExchanges)
	if err != nil {
		return nil, fmt.Errorf("failed to get Lowest price from database: %w", err)
	}

	if data == nil {
		return nil, fmt.Errorf("no price data found for symbol %s in current mode", validSymbol)
	}

	return data, nil
}

// GetHighestPriceByExchange returns the highest price for a symbol and specific exchange from last 30 records
func (s *PriceService) GetLowestPriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error) {
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

	// Get from database
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	data, err := s.pricesRepo.GetLowestPriceByExchangeFromLatestRecord(ctx, validSymbol, validExchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get lowest price by exchange from database: %w", err)
	}

	if data == nil {
		return nil, fmt.Errorf("no price data found for symbol %s on exchange %s", validSymbol, validExchange)
	}

	return data, nil
}

// GetHighestPrice returns the highest price for a symbol from last 30 records, filtered by current mode
func (s *PriceService) GetAveragePrice(ctx context.Context, symbol string) (*domain.MarketData, error) {
	// Validate symbol
	validSymbol, err := s.validateSymbol(symbol)
	if err != nil {
		return nil, err
	}

	// Get allowed exchanges for current mode
	allowedExchanges := s.getAllowedExchanges()
	if len(allowedExchanges) == 0 {
		return nil, fmt.Errorf("no exchanges available for current mode")
	}

	// Get from database
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	data, err := s.pricesRepo.GetAveragePriceFromLatestRecords(ctx, validSymbol, allowedExchanges)
	if err != nil {
		return nil, fmt.Errorf("failed to get average price from database: %w", err)
	}

	if data == nil {
		return nil, fmt.Errorf("no price data found for symbol %s in current mode", validSymbol)
	}

	return data, nil
}

// GetHighestPriceByExchange returns the highest price for a symbol and specific exchange from last 30 records
func (s *PriceService) GetAveragePriceByExchange(ctx context.Context, symbol, exchange string) (*domain.MarketData, error) {
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

	// Get from database
	if s.pricesRepo == nil {
		return nil, fmt.Errorf("prices repository not available")
	}

	data, err := s.pricesRepo.GetAveragePriceByExchangeFromLatestRecord(ctx, validSymbol, validExchange)
	if err != nil {
		return nil, fmt.Errorf("failed to get average price by exchange from database: %w", err)
	}

	if data == nil {
		return nil, fmt.Errorf("no price data found for symbol %s on exchange %s", validSymbol, validExchange)
	}

	return data, nil
}
