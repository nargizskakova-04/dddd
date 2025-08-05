// internal/adapters/repository/postgres/lowest_prices.go
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

	// Create placeholders for the IN clause
	placeholders := make([]string, len(allowedExchanges))
	args := make([]interface{}, len(allowedExchanges)+1)
	args[0] = symbol // First argument is the symbol

	for i, exchange := range allowedExchanges {
		placeholders[i] = fmt.Sprintf("$%d", i+2) // Start from $2 since $1 is symbol
		args[i+1] = exchange
	}

	// LIMIT 3: Get latest record from each of the 3 allowed exchanges
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
			return nil, nil // No data found
		}
		return nil, fmt.Errorf("failed to get lowest price from latest records: %w", err)
	}

	// ✅ ИСПРАВЛЕНО: Консистентность - используем секунды как в остальной системе
	marketData.Timestamp = timestamp.Unix()
	return &marketData, nil
}

// ✅ ИСПРАВЛЕНО: Теперь действительно берем 30 записей ИЛИ упрощаем логику для latest
// GetLowestPriceByExchangeFromLatestRecord returns the lowest min_price from the latest record of specific exchange
func (r *PricesRepository) GetLowestPriceByExchangeFromLatestRecord(ctx context.Context, symbol, exchange string) (*domain.MarketData, error) {
	// ✅ УПРОЩЕНО: Если нужна только последняя запись, то просто берем min_price из неё
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
			return nil, nil // No data found
		}
		return nil, fmt.Errorf("failed to get lowest price by exchange from latest record: %w", err)
	}

	// ✅ ИСПРАВЛЕНО: Консистентность
	marketData.Timestamp = timestamp.Unix()
	return &marketData, nil
}
