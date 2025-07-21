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
	"sync"
	"time"

	"cryptomarket/internal/core/domain"
	"cryptomarket/internal/core/port"
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
	mu        sync.RWMutex

	// Connection tracking
	connectionAttempts int
	lastConnectTime    time.Time
	messagesReceived   int
}

func NewLiveExchangeAdapter(host string, port int, name string) port.ExchangeAdapter {
	return &LiveExchangeAdapter{
		host:      host,
		port:      port,
		name:      name,
		isHealthy: false,
	}
}

func (l *LiveExchangeAdapter) Start(ctx context.Context) (<-chan domain.MarketData, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isRunning {
		slog.Warn("Adapter already running", "name", l.name)
		return l.dataChan, nil
	}

	slog.Info("=== Starting LiveExchangeAdapter ===",
		"name", l.name,
		"host", l.host,
		"port", l.port)

	// Reset state
	l.connectionAttempts = 0
	l.messagesReceived = 0
	l.isHealthy = false

	// Create fresh channels
	l.dataChan = make(chan domain.MarketData, 500)
	l.stopChan = make(chan struct{})

	// Connect to exchange
	if err := l.connect(); err != nil {
		slog.Error("Failed to connect during start",
			"name", l.name,
			"host", l.host,
			"port", l.port,
			"error", err)
		return nil, fmt.Errorf("failed to connect to exchange %s:%d: %w", l.host, l.port, err)
	}

	l.isRunning = true
	l.isHealthy = true

	// Start reading data in goroutine
	go l.readData(ctx)

	// Start reconnection handler
	go l.handleReconnection(ctx)

	slog.Info("LiveExchangeAdapter started successfully",
		"name", l.name,
		"host", l.host,
		"port", l.port)
	return l.dataChan, nil
}

func (l *LiveExchangeAdapter) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isRunning {
		return nil
	}

	slog.Info("=== Stopping LiveExchangeAdapter ===",
		"name", l.name,
		"messages_received", l.messagesReceived)

	l.isRunning = false
	l.isHealthy = false

	// Close stop channel if not already closed
	select {
	case <-l.stopChan:
		// Already closed
	default:
		close(l.stopChan)
	}

	// Close connection
	if l.conn != nil {
		l.conn.Close()
		slog.Info("Closed connection", "name", l.name)
	}

	// Close data channel safely
	if l.dataChan != nil {
		close(l.dataChan)
		l.dataChan = nil
	}

	slog.Info("LiveExchangeAdapter stopped",
		"name", l.name,
		"total_messages_received", l.messagesReceived)
	return nil
}

func (l *LiveExchangeAdapter) Name() string {
	return l.name
}

func (l *LiveExchangeAdapter) IsHealthy() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.isHealthy
}

func (l *LiveExchangeAdapter) connect() error {
	l.connectionAttempts++
	l.lastConnectTime = time.Now()

	address := fmt.Sprintf("%s:%d", l.host, l.port)
	slog.Info("Attempting connection",
		"name", l.name,
		"address", address,
		"attempt", l.connectionAttempts)

	// First check if port is reachable
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		slog.Error("Connection failed",
			"name", l.name,
			"address", address,
			"attempt", l.connectionAttempts,
			"error", err)

		// Check if it's a connection refused error (port not open)
		if strings.Contains(err.Error(), "connection refused") {
			slog.Error("Connection refused - target service not running?",
				"name", l.name,
				"address", address)
		}

		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	l.conn = conn
	slog.Info("Connection established successfully",
		"name", l.name,
		"address", address,
		"attempt", l.connectionAttempts,
		"local_addr", conn.LocalAddr(),
		"remote_addr", conn.RemoteAddr())

	return nil
}

func (l *LiveExchangeAdapter) readData(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in readData", "name", l.name, "panic", r)
		}
		slog.Info("readData goroutine ended", "name", l.name, "messages_received", l.messagesReceived)
	}()

	scanner := bufio.NewScanner(l.conn)
	lastLogTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			slog.Info("readData context cancelled", "name", l.name)
			return
		case <-l.stopChan:
			slog.Info("readData stop signal received", "name", l.name)
			return
		default:
			// Set read deadline to avoid blocking forever
			l.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					slog.Error("Scanner error", "name", l.name, "error", err)
				} else {
					slog.Info("Scanner finished (connection closed by remote)", "name", l.name)
				}
				l.mu.Lock()
				l.isHealthy = false
				l.mu.Unlock()
				return
			}

			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			l.messagesReceived++

			// Log first few messages and then periodically
			if l.messagesReceived <= 5 || l.messagesReceived%100 == 0 || time.Since(lastLogTime) > 30*time.Second {
				slog.Info("Received message",
					"name", l.name,
					"count", l.messagesReceived,
					"message", line[:min(len(line), 100)]) // Truncate long messages
				lastLogTime = time.Now()
			}

			// Parse market data from line
			marketData, err := l.parseMarketData(line)
			if err != nil {
				slog.Warn("Failed to parse market data",
					"name", l.name,
					"line", line[:min(len(line), 100)],
					"error", err)
				continue
			}

			// Verify exchange name is set correctly
			if marketData.Exchange != l.name {
				slog.Debug("Updated exchange name in market data",
					"from", marketData.Exchange,
					"to", l.name)
				marketData.Exchange = l.name
			}

			// Send to data channel if running
			l.mu.RLock()
			running := l.isRunning
			dataChan := l.dataChan
			l.mu.RUnlock()

			if running && dataChan != nil {
				select {
				case dataChan <- *marketData:
					// Success
				case <-time.After(100 * time.Millisecond):
					slog.Warn("Data channel is full, dropping market data", "name", l.name)
				case <-l.stopChan:
					return
				}
			}
		}
	}
}

// Helper function since min doesn't exist in older Go versions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

	// Parse timestamp - handle both seconds and milliseconds
	if timestamp, ok := data["timestamp"].(float64); ok {
		// If timestamp is in milliseconds (> year 2100), convert to seconds
		if timestamp > 4000000000 {
			marketData.Timestamp = int64(timestamp / 1000)
		} else {
			marketData.Timestamp = int64(timestamp)
		}
	} else if timestampStr, ok := data["timestamp"].(string); ok {
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timestamp: %w", err)
		}
		// If timestamp is in milliseconds, convert to seconds
		if timestamp > 4000000000 {
			marketData.Timestamp = timestamp / 1000
		} else {
			marketData.Timestamp = timestamp
		}
	} else {
		// Use current time if no timestamp provided
		marketData.Timestamp = time.Now().Unix()
	}

	return marketData, nil
}

func (l *LiveExchangeAdapter) handleReconnection(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	reconnectAttempts := 0

	for {
		select {
		case <-ctx.Done():
			slog.Info("Reconnection handler context cancelled", "name", l.name)
			return
		case <-l.stopChan:
			slog.Info("Reconnection handler stop signal received", "name", l.name)
			return
		case <-ticker.C:
			l.mu.RLock()
			healthy := l.isHealthy
			running := l.isRunning
			l.mu.RUnlock()

			if !healthy && running {
				reconnectAttempts++
				slog.Info("Connection unhealthy, attempting to reconnect",
					"name", l.name,
					"reconnect_attempt", reconnectAttempts,
					"total_connection_attempts", l.connectionAttempts)

				// Close existing connection
				if l.conn != nil {
					l.conn.Close()
				}

				// Try to reconnect
				if err := l.connect(); err != nil {
					slog.Error("Reconnection failed",
						"name", l.name,
						"reconnect_attempt", reconnectAttempts,
						"error", err)

					// Check if we should give up
					if reconnectAttempts > 10 {
						slog.Error("Too many reconnection failures, giving up",
							"name", l.name,
							"reconnect_attempts", reconnectAttempts)
						l.mu.Lock()
						l.isRunning = false
						l.mu.Unlock()
						return
					}
					continue
				}

				l.mu.Lock()
				l.isHealthy = true
				l.mu.Unlock()

				reconnectAttempts = 0
				slog.Info("Reconnected successfully",
					"name", l.name,
					"total_connection_attempts", l.connectionAttempts)

				// Restart reading data
				go l.readData(ctx)
			}
		}
	}
}
