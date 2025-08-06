package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cryptomarket/internal/core/domain"
)

type PricesRepository struct {
	db *sql.DB
}

func NewPricesRepository(db *sql.DB) *PricesRepository {
	return &PricesRepository{
		db: db,
	}
}
func (r *PricesRepository) InsertAggregatedPrice(ctx context.Context, aggregatedPrice domain.Prices) error {
	query := `
        INSERT INTO prices (pair_name, exchange, timestamp, average_price, min_price, max_price)
        VALUES ($1, $2, $3, $4, $5, $6)
    `

	timestamp := time.UnixMilli(aggregatedPrice.Timestamp)

	_, err := r.db.ExecContext(ctx, query,
		aggregatedPrice.PairName,
		aggregatedPrice.Exchange,
		timestamp,
		aggregatedPrice.AveragePrice,
		aggregatedPrice.MinPrice,
		aggregatedPrice.MaxPrice,
	)

	if err != nil {
		return fmt.Errorf("failed to insert aggregated price: %w", err)
	}

	return nil
}
func (r *PricesRepository) InsertAggregatedPrices(ctx context.Context, aggregatedPrices []domain.Prices) error {
	if len(aggregatedPrices) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
        INSERT INTO prices (pair_name, exchange, timestamp, average_price, min_price, max_price)
        VALUES ($1, $2, $3, $4, $5, $6)
    `

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, price := range aggregatedPrices {
		timestamp := time.UnixMilli(price.Timestamp)

		_, err := stmt.ExecContext(ctx,
			price.PairName,
			price.Exchange,
			timestamp,
			price.AveragePrice,
			price.MinPrice,
			price.MaxPrice,
		)
		if err != nil {
			return fmt.Errorf("failed to execute statement for %s/%s: %w", price.PairName, price.Exchange, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *PricesRepository) GetLatestAggregationTime(ctx context.Context) (time.Time, error) {
	query := `SELECT MAX(timestamp) FROM prices`

	var timestamp sql.NullTime
	err := r.db.QueryRowContext(ctx, query).Scan(&timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get latest aggregation time: %w", err)
	}

	if !timestamp.Valid {
		return time.Time{}, nil
	}

	return timestamp.Time, nil
}

func (r *PricesRepository) GetAggregatedPricesInRange(ctx context.Context, symbol, exchange string, from, to time.Time) ([]domain.Prices, error) {
	query := `
		SELECT pair_name, exchange, timestamp, average_price, min_price, max_price
		FROM prices
		WHERE pair_name = $1 AND exchange = $2 AND timestamp >= $3 AND timestamp <= $4
		ORDER BY timestamp ASC
	`

	rows, err := r.db.QueryContext(ctx, query, symbol, exchange, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to query aggregated prices: %w", err)
	}
	defer rows.Close()

	var prices []domain.Prices
	for rows.Next() {
		var price domain.Prices
		err := rows.Scan(
			&price.PairName,
			&price.Exchange,
			&price.Timestamp,
			&price.AveragePrice,
			&price.MinPrice,
			&price.MaxPrice,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan price row: %w", err)
		}
		prices = append(prices, price)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over rows: %w", err)
	}

	return prices, nil
}

func (r *PricesRepository) HealthCheck(ctx context.Context) error {
	query := `SELECT 1`

	var result int
	err := r.db.QueryRowContext(ctx, query).Scan(&result)
	if err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}
