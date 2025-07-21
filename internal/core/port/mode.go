// internal/core/port/mode.go
package port

import "context"

type ModeService interface {
	// Switch to test mode (use synthetic data generator)
	SwitchToTestMode(ctx context.Context) error

	// Switch to live mode (connect to real exchanges)
	SwitchToLiveMode(ctx context.Context) error

	// Get current mode
	GetCurrentMode() string
}
