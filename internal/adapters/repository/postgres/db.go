package postgres

import (
	"database/sql"
	"errors"
	"fmt"

	"crypto/internal/config"
)

func NewDbConnInstance(cfg *config.Repository) (*sql.DB, error) {
	if cfg == nil {
		return nil, errors.New("Postgres configuration is nil")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUsername,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("Failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("Failed to ping database: %w", err)
	}

	return db, nil
}
