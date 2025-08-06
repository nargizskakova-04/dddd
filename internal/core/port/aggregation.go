// internal/core/port/aggregation.go
package port

import "context"

type AggregationService interface {
	// Start the aggregation service (runs every minute)
	Start(ctx context.Context) error

	// Stop the aggregation service
	Stop() error

	// Check if the service is running
	IsRunning() bool

	// Get statistics about the aggregation service
	GetStats() map[string]interface{}
}
