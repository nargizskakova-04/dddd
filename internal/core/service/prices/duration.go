// internal/core/service/prices/duration.go
package prices

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"
)

var (
	// Duration pattern: supports formats like "30s", "5m", "2h", "1d"
	durationPattern = regexp.MustCompile(`^(\d+)([smhd])$`)
)

// ParseDuration parses duration strings like "30s", "5m", "2h", "1d"
// Returns the duration and whether it should use cache (sub-minute) or database (minute+)
func ParseDuration(durationStr string) (time.Duration, bool, error) {
	if durationStr == "" {
		// Default to 1 minute if no duration specified
		return time.Minute, false, nil
	}

	// Try Go's standard duration parser first (supports "1h30m", "90s", etc.)
	if duration, err := time.ParseDuration(durationStr); err == nil {
		useCache := duration < time.Minute
		slog.Debug("Parsed duration using standard parser",
			"input", durationStr,
			"duration", duration,
			"use_cache", useCache)
		return duration, useCache, nil
	}

	// Try custom parser for simple formats like "30s", "5m"
	matches := durationPattern.FindStringSubmatch(durationStr)
	if len(matches) != 3 {
		return 0, false, fmt.Errorf("invalid duration format: %s (expected formats: 30s, 5m, 2h, 1d)", durationStr)
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false, fmt.Errorf("invalid duration value: %s", matches[1])
	}

	if value <= 0 {
		return 0, false, fmt.Errorf("duration value must be positive: %d", value)
	}

	var duration time.Duration
	unit := matches[2]

	switch unit {
	case "s":
		duration = time.Duration(value) * time.Second
	case "m":
		duration = time.Duration(value) * time.Minute
	case "h":
		duration = time.Duration(value) * time.Hour
	case "d":
		duration = time.Duration(value) * 24 * time.Hour
	default:
		return 0, false, fmt.Errorf("unsupported duration unit: %s", unit)
	}

	// Determine if we should use cache (Redis) or database (PostgreSQL)
	// Cache for anything less than 1 minute, database for 1 minute and above
	useCache := duration < time.Minute

	slog.Debug("Parsed duration using custom parser",
		"input", durationStr,
		"value", value,
		"unit", unit,
		"duration", duration,
		"use_cache", useCache)

	return duration, useCache, nil
}

// ValidateDuration validates that the duration is within acceptable limits
func ValidateDuration(duration time.Duration) error {
	// Set reasonable limits
	const (
		minDuration = time.Second
		maxDuration = 24 * time.Hour // 1 day max
	)

	if duration < minDuration {
		return fmt.Errorf("duration too short: minimum is %v", minDuration)
	}

	if duration > maxDuration {
		return fmt.Errorf("duration too long: maximum is %v", maxDuration)
	}

	return nil
}

// GetTimeRange calculates the start and end times for a given duration from now
func GetTimeRange(duration time.Duration) (start, end time.Time) {
	end = time.Now()
	start = end.Add(-duration)
	return start, end
}

// GetTimeRangeFromTimestamp calculates the start and end times for a given duration from a specific timestamp
func GetTimeRangeFromTimestamp(timestamp time.Time, duration time.Duration) (start, end time.Time) {
	end = timestamp
	start = end.Add(-duration)
	return start, end
}

// DurationInfo provides information about how a duration will be processed
type DurationInfo struct {
	Duration    time.Duration `json:"duration"`
	UseCache    bool          `json:"use_cache"`
	DataSource  string        `json:"data_source"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Description string        `json:"description"`
}

// GetDurationInfo returns detailed information about how a duration will be processed
func GetDurationInfo(durationStr string) (*DurationInfo, error) {
	duration, useCache, err := ParseDuration(durationStr)
	if err != nil {
		return nil, err
	}

	if err := ValidateDuration(duration); err != nil {
		return nil, err
	}

	start, end := GetTimeRange(duration)

	dataSource := "PostgreSQL (aggregated minute data)"
	description := "Using aggregated data from database"
	if useCache {
		dataSource = "Redis (real-time data)"
		description = "Using real-time data from cache"
	}

	return &DurationInfo{
		Duration:    duration,
		UseCache:    useCache,
		DataSource:  dataSource,
		StartTime:   start,
		EndTime:     end,
		Description: description,
	}, nil
}
