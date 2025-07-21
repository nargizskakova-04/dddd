// internal/core/port/health.go
package port

import "context"

type HealthService interface {
	// Get system health status
	GetSystemHealth(ctx context.Context) map[string]interface{}
}
