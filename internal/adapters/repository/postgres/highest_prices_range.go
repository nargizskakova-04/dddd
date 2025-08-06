package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"cryptomarket/internal/core/domain"
)

func (r *PricesRepository) GetHighestPriceInRange(ctx context.Context, symbol string, allowedExchanges []string, from, to time.Time) (*domain.MarketData, error) {
	if len(allowedExchanges) == 0 {
		return nil, fmt.Errorf("no allowed exchanges provided")
	}

	slog.Debug("Getting highest price in range from database",
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
		SELECT pair_name, exchange, timestamp, max_price
		FROM prices
		WHERE pair_name = $1 
		  AND timestamp >= $2 
		  AND timestamp <= $3 
		  AND exchange IN (%s)
		  AND max_price = (
		      SELECT MAX(max_price) 
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
		return nil, fmt.Errorf("failed to get highest price in range: %w", err)
	}

	marketData.Timestamp = timestamp.Unix()

	slog.Info("Found highest price in database",
		"symbol", symbol,
		"price", marketData.Price,
		"exchange", marketData.Exchange,
		"timestamp", timestamp.Format(time.RFC3339),
		"time_range", fmt.Sprintf("%s to %s", from.Format(time.RFC3339), to.Format(time.RFC3339)))

	return &marketData, nil
}

func (r *PricesRepository) GetHighestPriceInRangeByExchange(ctx context.Context, symbol, exchange string, from, to time.Time) (*domain.MarketData, error) {
	slog.Debug("Getting highest price in range by exchange from database",
		"symbol", symbol,
		"exchange", exchange,
		"from", from.Format(time.RFC3339),
		"to", to.Format(time.RFC3339),
		"duration", to.Sub(from))

	query := `
		SELECT pair_name, exchange, timestamp, max_price
		FROM prices
		WHERE pair_name = $1 
		  AND exchange = $2 
		  AND timestamp >= $3 
		  AND timestamp <= $4
		  AND max_price = (
		      SELECT MAX(max_price) 
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
		return nil, fmt.Errorf("failed to get highest price in range by exchange: %w", err)
	}

	marketData.Timestamp = timestamp.Unix()

	slog.Info("Found highest price in database for exchange",
		"symbol", symbol,
		"exchange", exchange,
		"price", marketData.Price,
		"timestamp", timestamp.Format(time.RFC3339),
		"time_range", fmt.Sprintf("%s to %s", from.Format(time.RFC3339), to.Format(time.RFC3339)))

	return &marketData, nil
}

func (r *PricesRepository) GetPriceDataStats(ctx context.Context, symbol string, exchanges []string) (map[string]interface{}, error) {
	if len(exchanges) == 0 {
		return nil, fmt.Errorf("no exchanges provided")
	}

	stats := make(map[string]interface{})

	placeholders := make([]string, len(exchanges))
	args := make([]interface{}, len(exchanges)+1)
	args[0] = symbol

	for i, exchange := range exchanges {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = exchange
	}

	query := fmt.Sprintf(`
		SELECT 
			COUNT(*) as total_records,
			MIN(timestamp) as earliest_time,
			MAX(timestamp) as latest_time,
			MIN(min_price) as lowest_price,
			MAX(max_price) as highest_price,
			AVG(average_price) as avg_price
		FROM prices
		WHERE pair_name = $1 AND exchange IN (%s)
	`, joinPlaceholders(placeholders))

	var totalRecords int
	var earliestTime, latestTime sql.NullTime
	var lowestPrice, highestPrice, avgPrice sql.NullFloat64

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&totalRecords,
		&earliestTime,
		&latestTime,
		&lowestPrice,
		&highestPrice,
		&avgPrice,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get price data stats: %w", err)
	}

	stats["symbol"] = symbol
	stats["exchanges"] = exchanges
	stats["total_records"] = totalRecords

	if earliestTime.Valid {
		stats["earliest_time"] = earliestTime.Time.Format(time.RFC3339)
	}
	if latestTime.Valid {
		stats["latest_time"] = latestTime.Time.Format(time.RFC3339)
	}
	if lowestPrice.Valid {
		stats["lowest_price"] = lowestPrice.Float64
	}
	if highestPrice.Valid {
		stats["highest_price"] = highestPrice.Float64
	}
	if avgPrice.Valid {
		stats["average_price"] = avgPrice.Float64
	}

	exchangeStats := make(map[string]interface{})
	for _, exchange := range exchanges {
		exchangeQuery := `
			SELECT 
				COUNT(*) as records,
				MIN(timestamp) as earliest,
				MAX(timestamp) as latest,
				MAX(max_price) as highest
			FROM prices
			WHERE pair_name = $1 AND exchange = $2
		`

		var records int
		var earliest, latest sql.NullTime
		var highest sql.NullFloat64

		err := r.db.QueryRowContext(ctx, exchangeQuery, symbol, exchange).Scan(
			&records,
			&earliest,
			&latest,
			&highest,
		)

		if err != nil {
			continue
		}

		exchangeStat := map[string]interface{}{
			"records": records,
		}

		if earliest.Valid {
			exchangeStat["earliest"] = earliest.Time.Format(time.RFC3339)
		}
		if latest.Valid {
			exchangeStat["latest"] = latest.Time.Format(time.RFC3339)
		}
		if highest.Valid {
			exchangeStat["highest_price"] = highest.Float64
		}

		exchangeStats[exchange] = exchangeStat
	}

	stats["per_exchange"] = exchangeStats

	return stats, nil
}
