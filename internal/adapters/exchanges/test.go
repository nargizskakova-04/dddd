package exchanges

import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"crypto/internal/core/domain"
	"crypto/internal/core/port"
)

// TestExchangeAdapter generates synthetic market data using the Generator pattern
type TestExchangeAdapter struct {
	name       string
	dataChan   chan domain.MarketData
	stopChan   chan struct{}
	isRunning  bool
	symbols    []string
	basePrices map[string]float64
	volatility map[string]float64
}

func NewTestExchangeAdapter(name string) port.ExchangeAdapter {
	return &TestExchangeAdapter{
		name:     name,
		dataChan: make(chan domain.MarketData, 100),
		stopChan: make(chan struct{}),
		symbols:  []string{"BTCUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT", "ETHUSDT"},
		basePrices: map[string]float64{
			"BTCUSDT":  96000.0, // Base price for Bitcoin
			"DOGEUSDT": 0.32,    // Base price for Dogecoin
			"TONUSDT":  5.45,    // Base price for Toncoin
			"SOLUSDT":  210.0,   // Base price for Solana
			"ETHUSDT":  3300.0,  // Base price for Ethereum
		},
		volatility: map[string]float64{
			"BTCUSDT":  0.02,  // 2% max change
			"DOGEUSDT": 0.05,  // 5% max change (more volatile)
			"TONUSDT":  0.04,  // 4% max change
			"SOLUSDT":  0.03,  // 3% max change
			"ETHUSDT":  0.025, // 2.5% max change
		},
	}
}

func (t *TestExchangeAdapter) Start(ctx context.Context) (<-chan domain.MarketData, error) {
	if t.isRunning {
		return t.dataChan, nil
	}

	t.isRunning = true

	// Start data generation in goroutines for each symbol
	for _, symbol := range t.symbols {
		go t.generateDataForSymbol(ctx, symbol)
	}

	slog.Info("Test exchange adapter started", "exchange", t.name)
	return t.dataChan, nil
}

func (t *TestExchangeAdapter) Stop() error {
	if !t.isRunning {
		return nil
	}

	t.isRunning = false

	// Close stop channel to signal all goroutines
	close(t.stopChan)

	// Close data channel
	close(t.dataChan)

	slog.Info("Test exchange adapter stopped", "exchange", t.name)
	return nil
}

func (t *TestExchangeAdapter) Name() string {
	return t.name
}

func (t *TestExchangeAdapter) IsHealthy() bool {
	return t.isRunning
}

func (t *TestExchangeAdapter) generateDataForSymbol(ctx context.Context, symbol string) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in generateDataForSymbol", "exchange", t.name, "symbol", symbol, "panic", r)
		}
	}()

	basePrice := t.basePrices[symbol]
	currentPrice := basePrice
	trend := 1.0 // 1.0 for upward, -1.0 for downward

	// Random seed for this symbol to make it more realistic
	source := rand.NewSource(time.Now().UnixNano() + int64(len(symbol)*len(t.name)))
	rng := rand.New(source)

	// Generate price updates every 100ms to 2 seconds
	ticker := time.NewTicker(time.Duration(100+rng.Intn(1900)) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopChan:
			return
		case <-ticker.C:
			// Generate realistic price movement
			currentPrice = t.generateNextPrice(rng, symbol, currentPrice, basePrice, &trend)

			marketData := domain.MarketData{
				Symbol:    symbol,
				Price:     currentPrice,
				Timestamp: time.Now().Unix(),
				Exchange:  t.name,
			}

			// Send to data channel if running
			if t.isRunning {
				select {
				case t.dataChan <- marketData:
				case <-time.After(50 * time.Millisecond):
					// Drop if channel is full
				}
			}

			// Vary the ticker interval for more realistic data
			ticker.Reset(time.Duration(100+rng.Intn(1900)) * time.Millisecond)
		}
	}
}

func (t *TestExchangeAdapter) generateNextPrice(rng *rand.Rand, symbol string, currentPrice, basePrice float64, trend *float64) float64 {
	// Get volatility for this symbol
	volatility := t.volatility[symbol]

	// Random walk with trend
	change := rng.NormFloat64() * volatility * currentPrice

	// Add trend bias (10% of the change is trend-based)
	trendStrength := 0.1
	change += change * trendStrength * (*trend)

	newPrice := currentPrice + change

	// Ensure price doesn't deviate too much from base price (within 20%)
	maxDeviation := basePrice * 0.2
	if newPrice > basePrice+maxDeviation {
		newPrice = basePrice + maxDeviation
		*trend = -1.0 // Reverse trend
	} else if newPrice < basePrice-maxDeviation {
		newPrice = basePrice - maxDeviation
		*trend = 1.0 // Reverse trend
	}

	// Ensure price is positive and has reasonable precision
	if newPrice <= 0 {
		newPrice = basePrice * 0.01 // 1% of base price as minimum
	}

	// Round to appropriate decimal places based on price level
	newPrice = t.roundPrice(newPrice)

	// Occasionally change trend (5% chance)
	if rng.Float64() < 0.05 {
		*trend = -(*trend)
	}

	// Add some market events simulation (rare spikes/dips)
	if rng.Float64() < 0.001 { // 0.1% chance
		eventMultiplier := 1.0 + (rng.Float64()-0.5)*0.1 // ±5% spike
		newPrice *= eventMultiplier
		newPrice = t.roundPrice(newPrice)
		slog.Debug("Market event simulated", "symbol", symbol, "exchange", t.name, "multiplier", eventMultiplier)
	}

	return newPrice
}

func (t *TestExchangeAdapter) roundPrice(price float64) float64 {
	if price > 1000 {
		// For high-value coins like BTC, round to 2 decimal places
		return math.Round(price*100) / 100
	} else if price > 10 {
		// For medium-value coins, round to 3 decimal places
		return math.Round(price*1000) / 1000
	} else {
		// For low-value coins, round to 4 decimal places
		return math.Round(price*10000) / 10000
	}
}

// GetBasePrices returns the base prices for all symbols (useful for testing)
func (t *TestExchangeAdapter) GetBasePrices() map[string]float64 {
	result := make(map[string]float64)
	for k, v := range t.basePrices {
		result[k] = v
	}
	return result
}

// GetVolatility returns the volatility settings for all symbols (useful for testing)
func (t *TestExchangeAdapter) GetVolatility() map[string]float64 {
	result := make(map[string]float64)
	for k, v := range t.volatility {
		result[k] = v
	}
	return result
}

// SetBasePrices allows updating base prices (useful for testing different scenarios)
func (t *TestExchangeAdapter) SetBasePrices(prices map[string]float64) {
	for symbol, price := range prices {
		if _, exists := t.basePrices[symbol]; exists && price > 0 {
			t.basePrices[symbol] = price
		}
	}
}

// SetVolatility allows updating volatility settings (useful for testing)
func (t *TestExchangeAdapter) SetVolatility(vol map[string]float64) {
	for symbol, v := range vol {
		if _, exists := t.volatility[symbol]; exists && v > 0 && v < 1 {
			t.volatility[symbol] = v
		}
	}
}
