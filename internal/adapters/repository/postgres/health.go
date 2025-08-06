package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"cryptomarket/internal/core/port"
)

type HealthRepository struct {
	db *sql.DB
}

func NewHealthRepository(db *sql.DB) port.HealthRepository {
	return &HealthRepository{
		db: db,
	}
}

func (h *HealthRepository) CheckDatabaseHealth(ctx context.Context) error {
	if h.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	if err := h.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	query := `SELECT 1`
	var result int
	if err := h.db.QueryRowContext(ctx, query).Scan(&result); err != nil {
		return fmt.Errorf("database query test failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("database query returned unexpected result: %d", result)
	}

	return nil
}
