// internal/adapters/repository/postgres/highest_prices.go
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cryptomarket/internal/core/domain"
)

// GetHighestPriceFromLast30Records returns the highest price for a symbol from the last 30 records
// filtered by allowed exchanges
func (r *PricesRepository) GetHighestPriceFromLast30Records(ctx context.Context, symbol string, allowedExchanges []string) (*domain.MarketData, error) {
	if len(allowedExchanges) == 0 {
		return nil, fmt.Errorf("no allowed exchanges provided")
	}

	// Create placeholders for the IN clause
	placeholders := make([]string, len(allowedExchanges))
	args := make([]interface{}, len(allowedExchanges)+1)
	args[0] = symbol // First argument is the symbol

	for i, exchange := range allowedExchanges {
		placeholders[i] = fmt.Sprintf("$%d", i+2) // Start from $2 since $1 is symbol
		args[i+1] = exchange
	}

	query := fmt.Sprintf(`
		WITH latest_records AS (
			SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
			FROM prices
			WHERE pair_name = $1 AND exchange IN (%s)
			ORDER BY timestamp DESC
			LIMIT 3
		)
		SELECT pair_name, exchange, timestamp, max_price
		FROM latest_records
		WHERE max_price = (SELECT MAX(max_price) FROM latest_records)
		ORDER BY timestamp DESC
		LIMIT 1
	`, fmt.Sprintf("%s", placeholders[0]))

	// Build the full query with all placeholders
	query = fmt.Sprintf(`
		WITH latest_records AS (
			SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
			FROM prices
			WHERE pair_name = $1 AND exchange IN (%s)
			ORDER BY timestamp DESC
			LIMIT 3
		)
		SELECT pair_name, exchange, timestamp, max_price
		FROM latest_records
		WHERE max_price = (SELECT MAX(max_price) FROM latest_records)
		ORDER BY timestamp DESC
		LIMIT 1
	`, joinPlaceholders(placeholders))

	var marketData domain.MarketData
	var timestamp time.Time

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&marketData.Symbol,
		&marketData.Exchange,
		&timestamp,
		&marketData.Price,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No data found
		}
		return nil, fmt.Errorf("failed to get highest price from last 30 records: %w", err)
	}

	// Convert timestamp to milliseconds
	marketData.Timestamp = timestamp.UnixMilli()

	return &marketData, nil
}

// GetHighestPriceByExchangeFromLast30Records returns the highest price for a symbol and specific exchange
// from the last 30 records
func (r *PricesRepository) GetHighestPriceByExchangeFromLast30Records(ctx context.Context, symbol, exchange string) (*domain.MarketData, error) {
	query := `
		WITH latest_records AS (
			SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
			FROM prices
			WHERE pair_name = $1 AND exchange = $2
			ORDER BY timestamp DESC
			LIMIT 1
		)
		SELECT pair_name, exchange, timestamp, max_price
		FROM latest_records
		WHERE max_price = (SELECT MAX(max_price) FROM latest_records)
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var marketData domain.MarketData
	var timestamp time.Time

	err := r.db.QueryRowContext(ctx, query, symbol, exchange).Scan(
		&marketData.Symbol,
		&marketData.Exchange,
		&timestamp,
		&marketData.Price,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No data found
		}
		return nil, fmt.Errorf("failed to get highest price by exchange from last 30 records: %w", err)
	}

	// Convert timestamp to milliseconds
	marketData.Timestamp = timestamp.UnixMilli()

	return &marketData, nil
}

// Helper function to join placeholders for IN clause
func joinPlaceholders(placeholders []string) string {
	result := ""
	for i, placeholder := range placeholders {
		if i > 0 {
			result += ", "
		}
		result += placeholder
	}
	return result
}
