// internal/core/port/health.go
package port

import "context"

type HealthRepository interface {
	// CheckDatabaseHealth checks if the database connection is healthy
	CheckDatabaseHealth(ctx context.Context) error
}

type HealthService interface {
	// GetSystemHealth returns overall system health status
	GetSystemHealth(ctx context.Context) map[string]interface{}

	// GetDetailedHealth returns detailed health information for all components
	GetDetailedHealth(ctx context.Context) map[string]interface{}

	// IsHealthy returns true if all critical systems are healthy
	IsHealthy(ctx context.Context) bool
}
