package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cryptomarket/internal/core/domain"
)

func (r *PricesRepository) GetLowestPriceFromLatestRecords(ctx context.Context, symbol string, allowedExchanges []string) (*domain.MarketData, error) {
	if len(allowedExchanges) == 0 {
		return nil, fmt.Errorf("no allowed exchanges provided")
	}

	placeholders := make([]string, len(allowedExchanges))
	args := make([]interface{}, len(allowedExchanges)+1)
	args[0] = symbol

	for i, exchange := range allowedExchanges {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
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
		SELECT pair_name, exchange, timestamp, min_price
		FROM latest_records
		WHERE min_price = (SELECT MIN(min_price) FROM latest_records)
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
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get lowest price from latest records: %w", err)
	}

	marketData.Timestamp = timestamp.Unix()
	return &marketData, nil
}

func (r *PricesRepository) GetLowestPriceByExchangeFromLatestRecord(ctx context.Context, symbol, exchange string) (*domain.MarketData, error) {
	query := `
		SELECT pair_name, exchange, timestamp, min_price
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
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get lowest price by exchange from latest record: %w", err)
	}

	marketData.Timestamp = timestamp.Unix()
	return &marketData, nil
}
