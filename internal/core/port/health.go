// internal/core/port/health.go
package port

import "context"

type HealthRepository interface {
	CheckDatabaseHealth(ctx context.Context) error
}

type HealthService interface {
	GetSystemHealth(ctx context.Context) map[string]interface{}

	GetDetailedHealth(ctx context.Context) map[string]interface{}

	IsHealthy(ctx context.Context) bool
}
