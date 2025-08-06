package port

import "context"

type AggregationService interface {
	Start(ctx context.Context) error

	Stop() error

	IsRunning() bool

	GetStats() map[string]interface{}
}
