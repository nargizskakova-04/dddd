package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"cryptomarket/internal/core/domain"
)

func (r *PricesRepository) GetLowestPriceInRange(ctx context.Context, symbol string, allowedExchanges []string, from, to time.Time) (*domain.MarketData, error) {
	if len(allowedExchanges) == 0 {
		return nil, fmt.Errorf("no allowed exchanges provided")
	}

	slog.Debug("Getting lowest price in range from database",
		"symbol", symbol,
		"exchanges", allowedExchanges,
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339),
		"duration", to.Sub(from))

	placeholders := make([]string, len(allowedExchanges))
	args := make([]interface{}, len(allowedExchanges)+3)
	args[0] = symbol
	args[1] = from
	args[2] = to

	for i, exchange := range allowedExchanges {
		placeholders[i] = fmt.Sprintf("$%d", i+4)
		args[i+3] = exchange
	}

	query := fmt.Sprintf(`
		SELECT pair_name, exchange, timestamp, min_price
		FROM prices
		WHERE pair_name = $1 
		  AND timestamp >= $2 
		  AND timestamp <= $3 
		  AND exchange IN (%s)
		  AND min_price = (
		      SELECT MIN(min_price) 
		      FROM prices 
		      WHERE pair_name = $1 
		        AND timestamp >= $2 
		        AND timestamp <= $3 
		        AND exchange IN (%s)
		  )
		ORDER BY timestamp DESC
		LIMIT 1
	`, joinPlaceholders(placeholders), joinPlaceholders(placeholders))

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
			slog.Debug("No price data found in database",
				"symbol", symbol,
				"exchanges", allowedExchanges,
				"time_range", fmt.Sprintf("%s to %s", from.Format(time.RFC3339), to.Format(time.RFC3339)))
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get lowest price in range: %w", err)
	}

	marketData.Timestamp = timestamp.Unix()

	slog.Info("Found lowest price in database",
		"symbol", symbol,
		"price", marketData.Price,
		"exchange", marketData.Exchange,
		"timestamp", timestamp.Format(time.RFC3339),
		"time_range", fmt.Sprintf("%s to %s", from.Format(time.RFC3339), to.Format(time.RFC3339)))

	return &marketData, nil
}

func (r *PricesRepository) GetLowestPriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error) {
	slog.Debug("Getting lowest price in range by exchange from database",
		"symbol", symbol,
		"exchange", exchange,
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339),
		"duration", to.Sub(from))

	query := `
		SELECT pair_name, exchange, timestamp, min_price
		FROM prices
		WHERE pair_name = $1 
		  AND exchange = $2 
		  AND timestamp >= $3 
		  AND timestamp <= $4
		  AND min_price = (
		      SELECT MIN(min_price) 
		      FROM prices 
		      WHERE pair_name = $1 
		        AND exchange = $2 
		        AND timestamp >= $3 
		        AND timestamp <= $4
		  )
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var marketData domain.MarketData
	var timestamp time.Time

	err := r.db.QueryRowContext(ctx, query, symbol, exchange, from, to).Scan(
		&marketData.Symbol,
		&marketData.Exchange,
		&timestamp,
		&marketData.Price,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			slog.Debug("No price data found in database for exchange",
				"symbol", symbol,
				"exchange", exchange,
				"time_range", fmt.Sprintf("%s to %s", from.Format(time.RFC3339), to.Format(time.RFC3339)))
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get lowest price in range by exchange: %w", err)
	}

	marketData.Timestamp = timestamp.Unix()

	slog.Info("Found lowest price in database for exchange",
		"symbol", symbol,
		"exchange", exchange,
		"price", marketData.Price,
		"timestamp", timestamp.Format(time.RFC3339),
		"time_range", fmt.Sprintf("%s to %s", from.Format(time.RFC3339), to.Format(time.RFC3339)))

	return &marketData, nil
}
