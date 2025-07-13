package exchanges

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"crypto/internal/core/domain"
	"crypto/internal/core/port"
)

// LiveExchangeAdapter connects to real exchange programs via TCP
type LiveExchangeAdapter struct {
	host      string
	port      int
	name      string
	conn      net.Conn
	dataChan  chan domain.MarketData
	stopChan  chan struct{}
	isRunning bool
	isHealthy bool
}

func NewLiveExchangeAdapter(host string, port int, name string) port.ExchangeAdapter {
	return &LiveExchangeAdapter{
		host:      host,
		port:      port,
		name:      name,
		dataChan:  make(chan domain.MarketData, 100),
		stopChan:  make(chan struct{}),
		isHealthy: false,
	}
}

func (l *LiveExchangeAdapter) Start(ctx context.Context) (<-chan domain.MarketData, error) {
	if l.isRunning {
		return l.dataChan, nil
	}

	// Connect to exchange
	if err := l.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to exchange %s: %w", l.name, err)
	}

	l.isRunning = true
	l.isHealthy = true

	// Start reading data in goroutine
	go l.readData(ctx)

	// Start reconnection handler
	go l.handleReconnection(ctx)

	slog.Info("Live exchange adapter started", "exchange", l.name, "port", l.port)
	return l.dataChan, nil
}

func (l *LiveExchangeAdapter) Stop() error {
	if !l.isRunning {
		return nil
	}

	l.isRunning = false
	l.isHealthy = false

	// Close stop channel
	close(l.stopChan)

	// Close connection
	if l.conn != nil {
		l.conn.Close()
	}

	// Close data channel
	close(l.dataChan)

	slog.Info("Live exchange adapter stopped", "exchange", l.name)
	return nil
}

func (l *LiveExchangeAdapter) Name() string {
	return l.name
}

func (l *LiveExchangeAdapter) IsHealthy() bool {
	return l.isHealthy
}

func (l *LiveExchangeAdapter) connect() error {
	address := fmt.Sprintf("%s:%d", l.host, l.port)
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	l.conn = conn
	return nil
}

func (l *LiveExchangeAdapter) readData(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in readData", "exchange", l.name, "panic", r)
		}
	}()

	scanner := bufio.NewScanner(l.conn)

	for l.isRunning {
		select {
		case <-ctx.Done():
			return
		case <-l.stopChan:
			return
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					slog.Error("Scanner error", "exchange", l.name, "error", err)
				}
				l.isHealthy = false
				return
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Parse market data from line
			marketData, err := l.parseMarketData(line)
			if err != nil {
				slog.Warn("Failed to parse market data", "exchange", l.name, "line", line, "error", err)
				continue
			}

			// Send to data channel if running
			if l.isRunning {
				select {
				case l.dataChan <- *marketData:
				case <-time.After(100 * time.Millisecond):
					slog.Warn("Data channel is full, dropping market data", "exchange", l.name)
				}
			}
		}
	}
}

func (l *LiveExchangeAdapter) parseMarketData(line string) (*domain.MarketData, error) {
	// Try to parse as JSON first
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(line), &jsonData); err == nil {
		return l.parseJSONMarketData(jsonData)
	}

	// If not JSON, try to parse as simple format
	// Expected format: "SYMBOL:PRICE" or "SYMBOL PRICE"
	parts := strings.Fields(line)
	if len(parts) < 2 {
		// Try colon separator
		parts = strings.Split(line, ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid line format: %s", line)
		}
	}

	symbol := strings.TrimSpace(parts[0])
	priceStr := strings.TrimSpace(parts[1])

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price %s: %w", priceStr, err)
	}

	return &domain.MarketData{
		Symbol:    symbol,
		Price:     price,
		Timestamp: time.Now().Unix(),
		Exchange:  l.name,
	}, nil
}

func (l *LiveExchangeAdapter) parseJSONMarketData(data map[string]interface{}) (*domain.MarketData, error) {
	marketData := &domain.MarketData{
		Exchange: l.name,
	}

	// Parse symbol
	if symbol, ok := data["symbol"].(string); ok {
		marketData.Symbol = symbol
	} else if symbol, ok := data["pair"].(string); ok {
		marketData.Symbol = symbol
	} else {
		return nil, fmt.Errorf("missing symbol in JSON data")
	}

	// Parse price
	if price, ok := data["price"].(float64); ok {
		marketData.Price = price
	} else if priceStr, ok := data["price"].(string); ok {
		price, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse price from string: %w", err)
		}
		marketData.Price = price
	} else {
		return nil, fmt.Errorf("missing price in JSON data")
	}

	// Parse timestamp
	if timestamp, ok := data["timestamp"].(float64); ok {
		marketData.Timestamp = int64(timestamp)
	} else if timestampStr, ok := data["timestamp"].(string); ok {
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}
		marketData.Timestamp = timestamp
	} else {
		// Use current time if no timestamp provided
		marketData.Timestamp = time.Now().Unix()
	}

	return marketData, nil
}

func (l *LiveExchangeAdapter) handleReconnection(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-l.stopChan:
			return
		case <-ticker.C:
			if !l.isHealthy && l.isRunning {
				slog.Info("Attempting to reconnect", "exchange", l.name)

				// Close existing connection
				if l.conn != nil {
					l.conn.Close()
				}

				// Try to reconnect
				if err := l.connect(); err != nil {
					slog.Error("Failed to reconnect", "exchange", l.name, "error", err)
					continue
				}

				l.isHealthy = true
				slog.Info("Reconnected successfully", "exchange", l.name)

				// Restart reading data
				go l.readData(ctx)
			}
		}
	}
}
