package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cryptomarket/internal/core/domain"
)

func (r *PricesRepository) GetAveragePriceFromLatestRecords(ctx context.Context, symbol string, allowedExchanges []string) (*domain.MarketData, error) {
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

	// FIXED: Get latest record from each allowed exchange and calculate average
	query := fmt.Sprintf(`
		WITH latest_records AS (
			SELECT DISTINCT ON (exchange) 
				pair_name, exchange, timestamp, average_price, min_price, max_price
			FROM prices
			WHERE pair_name = $1 AND exchange IN (%s)
			ORDER BY exchange, timestamp DESC
		)
		SELECT 
			pair_name,
			'multiple' as exchange,  -- Indicate this is from multiple exchanges
			MAX(timestamp) as timestamp,  -- Use most recent timestamp
			AVG(average_price) as calculated_average  -- Calculate actual average
		FROM latest_records
		GROUP BY pair_name
	`, joinPlaceholders(placeholders))

	var marketData domain.MarketData
	var timestamp time.Time
	var avgPrice float64

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&marketData.Symbol,
		&marketData.Exchange,
		&timestamp,
		&avgPrice,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No data found
		}
		return nil, fmt.Errorf("failed to get average price from latest records: %w", err)
	}

	// FIXED: Use the calculated average price
	marketData.Price = avgPrice
	marketData.Timestamp = timestamp.Unix()

	return &marketData, nil
}

// FIXED: GetAveragePriceByExchangeFromLatestRecord - simplified to just return the average_price from latest record
func (r *PricesRepository) GetAveragePriceByExchangeFromLatestRecord(ctx context.Context, symbol, exchange string) (*domain.MarketData, error) {
	// FIXED: Simply get the average_price from the latest record for this exchange
	query := `
		SELECT pair_name, exchange, timestamp, average_price
		FROM prices
		WHERE pair_name = $1 AND exchange = $2
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
		return nil, fmt.Errorf("failed to get average price by exchange from latest record: %w", err)
	}

	// FIXED: Consistency - use Unix seconds
	marketData.Timestamp = timestamp.Unix()
	return &marketData, nil
}
