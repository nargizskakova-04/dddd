package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// MarketData represents a single price update
type MarketData struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
}

// ExchangeServer represents a single exchange server
type ExchangeServer struct {
	name       string
	port       int
	listener   net.Listener
	clients    map[net.Conn]bool
	clientsMux sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// TestGenerator manages multiple exchange servers
type TestGenerator struct {
	servers  []*ExchangeServer
	symbols  []string
	baseData map[string]*SymbolData
	ctx      context.Context
	cancel   context.CancelFunc
}

// SymbolData holds base price and volatility for each symbol
type SymbolData struct {
	BasePrice    float64
	CurrentPrice float64
	Volatility   float64      // percentage as decimal (0.02 = 2%)
	Trend        float64      // 1.0 for up, -1.0 for down
	mu           sync.RWMutex // Add mutex for thread safety
}

func main() {
	var (
		port1    = flag.Int("port1", 50101, "Port for test-exchange1")
		port2    = flag.Int("port2", 50102, "Port for test-exchange2")
		port3    = flag.Int("port3", 50103, "Port for test-exchange3")
		helpFlag = flag.Bool("help", false, "Show help message")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  test-generator [--port1 <N>] [--port2 <N>] [--port3 <N>]\n")
		fmt.Fprintf(os.Stderr, "  test-generator --help\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  --port1 N    Port for test-exchange1 (default: 50101)\n")
		fmt.Fprintf(os.Stderr, "  --port2 N    Port for test-exchange2 (default: 50102)\n")
		fmt.Fprintf(os.Stderr, "  --port3 N    Port for test-exchange3 (default: 50103)\n")
	}

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	// Setup logging
	slog.Info("Starting Test Exchange Generator...")

	// Create generator
	generator := NewTestGenerator()

	// Create servers
	ports := []int{*port1, *port2, *port3}
	names := []string{"test-exchange1", "test-exchange2", "test-exchange3"}

	for i, port := range ports {
		server, err := NewExchangeServer(names[i], port, generator.ctx)
		if err != nil {
			slog.Error("Failed to create exchange server", "name", names[i], "port", port, "error", err)
			os.Exit(1)
		}
		generator.servers = append(generator.servers, server)
	}

	// Start all servers
	for _, server := range generator.servers {
		if err := server.Start(); err != nil {
			slog.Error("Failed to start server", "name", server.name, "error", err)
			os.Exit(1)
		}
	}

	// Start data generation
	generator.StartDataGeneration()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("Test Generator started successfully", "ports", ports)
	fmt.Printf("Test Generator running on ports: %v\n", ports)
	fmt.Printf("Press Ctrl+C to stop...\n\n")

	// Wait for shutdown signal
	<-sigChan
	slog.Info("Shutting down...")

	// Cleanup
	generator.Shutdown()
	slog.Info("Test Generator stopped")
}

// NewTestGenerator creates a new test generator
func NewTestGenerator() *TestGenerator {
	ctx, cancel := context.WithCancel(context.Background())

	symbols := []string{"BTCUSDT", "DOGEUSDT", "TONUSDT", "SOLUSDT", "ETHUSDT"}

	// Initialize base data for each symbol (using realistic crypto prices)
	baseData := map[string]*SymbolData{
		"BTCUSDT": {
			BasePrice:    96000.0,
			CurrentPrice: 96000.0,
			Volatility:   0.02, // 2%
			Trend:        1.0,
		},
		"DOGEUSDT": {
			BasePrice:    0.32,
			CurrentPrice: 0.32,
			Volatility:   0.05, // 5% (more volatile)
			Trend:        1.0,
		},
		"TONUSDT": {
			BasePrice:    5.45,
			CurrentPrice: 5.45,
			Volatility:   0.04, // 4%
			Trend:        1.0,
		},
		"SOLUSDT": {
			BasePrice:    210.0,
			CurrentPrice: 210.0,
			Volatility:   0.03, // 3%
			Trend:        1.0,
		},
		"ETHUSDT": {
			BasePrice:    3300.0,
			CurrentPrice: 3300.0,
			Volatility:   0.025, // 2.5%
			Trend:        1.0,
		},
	}

	return &TestGenerator{
		servers:  make([]*ExchangeServer, 0),
		symbols:  symbols,
		baseData: baseData,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// NewExchangeServer creates a new exchange server
func NewExchangeServer(name string, port int, parentCtx context.Context) (*ExchangeServer, error) {
	ctx, cancel := context.WithCancel(parentCtx)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	return &ExchangeServer{
		name:     name,
		port:     port,
		listener: listener,
		clients:  make(map[net.Conn]bool),
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Start starts the exchange server
func (s *ExchangeServer) Start() error {
	go s.acceptConnections()
	slog.Info("Exchange server started", "name", s.name, "port", s.port)
	return nil
}

// acceptConnections handles incoming client connections
func (s *ExchangeServer) acceptConnections() {
	defer s.listener.Close()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Set accept timeout to avoid blocking forever
			s.listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))

			conn, err := s.listener.Accept()
			if err != nil {
				// Check if it's a timeout or context cancellation
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				if !strings.Contains(err.Error(), "use of closed network connection") {
					slog.Error("Failed to accept connection", "server", s.name, "error", err)
				}
				continue
			}

			// Add client
			s.clientsMux.Lock()
			s.clients[conn] = true
			clientCount := len(s.clients)
			s.clientsMux.Unlock()

			slog.Info("Client connected", "server", s.name, "clients", clientCount, "addr", conn.RemoteAddr())

			// Handle client disconnection
			go s.handleClient(conn)
		}
	}
}

// handleClient manages a single client connection
func (s *ExchangeServer) handleClient(conn net.Conn) {
	defer func() {
		conn.Close()
		s.clientsMux.Lock()
		delete(s.clients, conn)
		clientCount := len(s.clients)
		s.clientsMux.Unlock()
		slog.Info("Client disconnected", "server", s.name, "clients", clientCount)
	}()

	// Keep connection alive until context is cancelled
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// Set a read deadline to detect disconnections
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			// Try to read from connection to detect if client disconnected
			buffer := make([]byte, 1024)
			_, err := conn.Read(buffer)
			if err != nil {
				// Client disconnected or error occurred
				return
			}
		}
	}
}

// BroadcastData sends market data to all connected clients
func (s *ExchangeServer) BroadcastData(data MarketData) {
	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	if len(s.clients) == 0 {
		return
	}

	// Convert to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("Failed to marshal data", "server", s.name, "error", err)
		return
	}

	jsonData = append(jsonData, '\n') // Add newline as delimiter

	// Send to all clients
	disconnectedClients := make([]net.Conn, 0)

	for client := range s.clients {
		// Set write deadline to avoid blocking
		client.SetWriteDeadline(time.Now().Add(5 * time.Second))

		if _, err := client.Write(jsonData); err != nil {
			// Client disconnected, mark for removal
			disconnectedClients = append(disconnectedClients, client)
		}
	}

	// Remove disconnected clients outside the read lock
	if len(disconnectedClients) > 0 {
		s.clientsMux.RUnlock()
		s.clientsMux.Lock()
		for _, client := range disconnectedClients {
			delete(s.clients, client)
			client.Close()
		}
		s.clientsMux.Unlock()
		s.clientsMux.RLock()
	}
}

// GetClientCount returns the number of connected clients
func (s *ExchangeServer) GetClientCount() int {
	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()
	return len(s.clients)
}

// StartDataGeneration starts generating market data for all servers
func (g *TestGenerator) StartDataGeneration() {
	// Generate data for each symbol on each server
	for _, server := range g.servers {
		for _, symbol := range g.symbols {
			go g.generateDataForSymbol(server, symbol)
		}
	}

	// Start stats reporting
	go g.reportStats()
}

// generateDataForSymbol generates realistic price data for a specific symbol
func (g *TestGenerator) generateDataForSymbol(server *ExchangeServer, symbol string) {
	symbolData := g.baseData[symbol]
	if symbolData == nil {
		slog.Error("No base data for symbol", "symbol", symbol)
		return
	}

	// Create unique random source for this symbol/server combination
	seed := time.Now().UnixNano() + int64(len(symbol)*server.port)
	rng := rand.New(rand.NewSource(seed))

	// Variable interval between updates (500ms to 3000ms for more realistic intervals)
	ticker := time.NewTicker(time.Duration(500+rng.Intn(2500)) * time.Millisecond)
	defer ticker.Stop()

	messageCount := 0

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			// Generate next price
			newPrice := g.generateNextPrice(rng, symbolData)

			// Update current price thread-safely
			symbolData.mu.Lock()
			symbolData.CurrentPrice = newPrice
			symbolData.mu.Unlock()

			// Create market data
			data := MarketData{
				Symbol:    symbol,
				Price:     newPrice,
				Timestamp: time.Now().UnixMilli(), // Unix timestamp in milliseconds
			}

			// Broadcast to clients
			server.BroadcastData(data)

			messageCount++
			if messageCount%50 == 0 {
				slog.Debug("Generated messages", "server", server.name, "symbol", symbol, "count", messageCount, "price", newPrice)
			}

			// Randomize next interval
			ticker.Reset(time.Duration(500+rng.Intn(2500)) * time.Millisecond)
		}
	}
}

// generateNextPrice creates the next price using realistic market movements
func (g *TestGenerator) generateNextPrice(rng *rand.Rand, symbolData *SymbolData) float64 {
	symbolData.mu.Lock()
	defer symbolData.mu.Unlock()

	// Random walk with trend
	change := rng.NormFloat64() * symbolData.Volatility * symbolData.CurrentPrice

	// Add trend bias (10% of the change is trend-based)
	trendStrength := 0.1
	change += change * trendStrength * symbolData.Trend

	newPrice := symbolData.CurrentPrice + change

	// Keep price within reasonable bounds (±20% from base price)
	maxDeviation := symbolData.BasePrice * 0.2
	if newPrice > symbolData.BasePrice+maxDeviation {
		newPrice = symbolData.BasePrice + maxDeviation
		symbolData.Trend = -1.0 // Reverse trend
	} else if newPrice < symbolData.BasePrice-maxDeviation {
		newPrice = symbolData.BasePrice - maxDeviation
		symbolData.Trend = 1.0 // Reverse trend
	}

	// Ensure positive price
	if newPrice <= 0 {
		newPrice = symbolData.BasePrice * 0.01 // 1% of base price as minimum
	}

	// Round to appropriate precision
	newPrice = g.roundPrice(newPrice)

	// Occasionally change trend (5% chance)
	if rng.Float64() < 0.05 {
		symbolData.Trend = -symbolData.Trend
	}

	// Rare market events (0.1% chance for larger moves)
	if rng.Float64() < 0.001 {
		eventMultiplier := 1.0 + (rng.Float64()-0.5)*0.1 // ±5% spike
		newPrice *= eventMultiplier
		newPrice = g.roundPrice(newPrice)
		slog.Debug("Market event simulated", "symbol", symbolData, "multiplier", eventMultiplier)
	}

	return newPrice
}

// roundPrice rounds price to appropriate decimal places based on value
func (g *TestGenerator) roundPrice(price float64) float64 {
	if price > 1000 {
		// High-value coins (like BTC): 2 decimal places
		return math.Round(price*100) / 100
	} else if price > 10 {
		// Medium-value coins: 3 decimal places
		return math.Round(price*1000) / 1000
	} else {
		// Low-value coins: 6 decimal places
		return math.Round(price*1000000) / 1000000
	}
}

// reportStats periodically logs statistics
func (g *TestGenerator) reportStats() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			totalClients := 0
			for _, server := range g.servers {
				clientCount := server.GetClientCount()
				totalClients += clientCount
				slog.Info("Server stats",
					"name", server.name,
					"port", server.port,
					"clients", clientCount)
			}
			slog.Info("Total clients connected", "count", totalClients)
		}
	}
}

// Shutdown gracefully shuts down all servers
func (g *TestGenerator) Shutdown() {
	g.cancel()

	// Close all server listeners
	for _, server := range g.servers {
		if server.listener != nil {
			server.listener.Close()
		}
	}

	// Give time for connections to close
	time.Sleep(1 * time.Second)
}
