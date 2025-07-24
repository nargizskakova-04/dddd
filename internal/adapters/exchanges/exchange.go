// Package exchanges provides implementations for connecting to cryptocurrency exchanges
// and generating synthetic market data for testing purposes.
package exchanges

import (
	"fmt"

	"cryptomarket/internal/core/port"
)

// Exchange types constants
const (
	ExchangeTypeLive = "live"
	ExchangeTypeTest = "test"
)

// Default exchange names
const (
	Exchange1Name     = "exchange1"
	Exchange2Name     = "exchange2"
	Exchange3Name     = "exchange3"
	TestExchange1Name = "test-exchange1"
	TestExchange2Name = "test-exchange2"
	TestExchange3Name = "test-exchange3"
)

// Default connection settings
const (
	DefaultHost = "127.0.0.1"
	LivePort1   = 40101
	LivePort2   = 40102
	LivePort3   = 40103
	TestPort1   = 50101
	TestPort2   = 50102
	TestPort3   = 50103
)

// ExchangeConfig holds configuration for creating exchange adapters
type ExchangeConfig struct {
	Name string
	Host string
	Port int
	Type string // "live" or "test"
}

// CreateLiveExchangeAdapters creates all three live exchange adapters with default settings
func CreateLiveExchangeAdapters() []port.ExchangeAdapter {
	return []port.ExchangeAdapter{
		NewLiveExchangeAdapter(DefaultHost, LivePort1, Exchange1Name),
		NewLiveExchangeAdapter(DefaultHost, LivePort2, Exchange2Name),
		NewLiveExchangeAdapter(DefaultHost, LivePort3, Exchange3Name),
	}
}

// CreateTestExchangeAdapters creates all three test exchange adapters
func CreateTestExchangeAdapters() []port.ExchangeAdapter {
	return []port.ExchangeAdapter{
		// ✅ ИСПРАВЛЕНО: Передаем правильные хост и порты для тестовых серверов
		NewTestExchangeAdapter(DefaultHost, TestPort1, TestExchange1Name), // localhost:50101
		NewTestExchangeAdapter(DefaultHost, TestPort2, TestExchange2Name), // localhost:50102
		NewTestExchangeAdapter(DefaultHost, TestPort3, TestExchange3Name), // localhost:50103
	}
}

// CreateLiveExchangeAdapter creates a single live exchange adapter
func CreateLiveExchangeAdapter(config ExchangeConfig) port.ExchangeAdapter {
	return NewLiveExchangeAdapter(config.Host, config.Port, config.Name)
}

// CreateTestExchangeAdapter creates a single test exchange adapter
func CreateTestExchangeAdapter(config ExchangeConfig) port.ExchangeAdapter {
	return NewTestExchangeAdapter(config.Host, config.Port, config.Name)
}

// CreateExchangeAdapter creates an exchange adapter based on type
func CreateExchangeAdapter(config ExchangeConfig) port.ExchangeAdapter {
	switch config.Type {
	case ExchangeTypeLive:
		return CreateLiveExchangeAdapter(config)
	case ExchangeTypeTest:
		return CreateTestExchangeAdapter(config)
	default:
		// Default to test adapter for safety
		return CreateTestExchangeAdapter(config)
	}
}

// CreateExchangeAdaptersFromConfigs creates adapters from a list of configurations
func CreateExchangeAdaptersFromConfigs(configs []ExchangeConfig) []port.ExchangeAdapter {
	adapters := make([]port.ExchangeAdapter, 0, len(configs))

	for _, config := range configs {
		adapter := CreateExchangeAdapter(config)
		adapters = append(adapters, adapter)
	}

	return adapters
}

// GetDefaultLiveConfigs returns default configurations for live exchanges
func GetDefaultLiveConfigs() []ExchangeConfig {
	return []ExchangeConfig{
		{
			Name: Exchange1Name,
			Host: DefaultHost,
			Port: LivePort1,
			Type: ExchangeTypeLive,
		},
		{
			Name: Exchange2Name,
			Host: DefaultHost,
			Port: LivePort2,
			Type: ExchangeTypeLive,
		},
		{
			Name: Exchange3Name,
			Host: DefaultHost,
			Port: LivePort3,
			Type: ExchangeTypeLive,
		},
	}
}

// GetDefaultTestConfigs returns default configurations for test exchanges
func GetDefaultTestConfigs() []ExchangeConfig {
	return []ExchangeConfig{
		{
			Name: TestExchange1Name,
			Host: DefaultHost,
			Port: TestPort1,
			Type: ExchangeTypeTest,
		},
		{
			Name: TestExchange2Name,
			Host: DefaultHost,
			Port: TestPort2,
			Type: ExchangeTypeTest,
		},
		{
			Name: TestExchange3Name,
			Host: DefaultHost,
			Port: TestPort3,
			Type: ExchangeTypeTest,
		},
	}
}

// ValidateExchangeConfig validates an exchange configuration
func ValidateExchangeConfig(config ExchangeConfig) error {
	if config.Name == "" {
		return fmt.Errorf("exchange name cannot be empty")
	}

	if config.Type == ExchangeTypeLive {
		if config.Host == "" {
			return fmt.Errorf("host cannot be empty for live exchange")
		}
		if config.Port <= 0 || config.Port > 65535 {
			return fmt.Errorf("invalid port number: %d", config.Port)
		}
	}

	if config.Type != ExchangeTypeLive && config.Type != ExchangeTypeTest {
		return fmt.Errorf("invalid exchange type: %s", config.Type)
	}

	return nil
}

// SupportedSymbols returns the list of supported trading pairs
func SupportedSymbols() []string {
	return []string{
		"BTCUSDT",
		"DOGEUSDT",
		"TONUSDT",
		"SOLUSDT",
		"ETHUSDT",
	}
}

// IsSymbolSupported checks if a trading pair is supported
func IsSymbolSupported(symbol string) bool {
	supported := SupportedSymbols()
	for _, s := range supported {
		if s == symbol {
			return true
		}
	}
	return false
}
