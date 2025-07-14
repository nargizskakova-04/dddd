package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"cryptomarket/internal/app"
)

func main() {
	slog.Info("Starting MarketFlow application...")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start application in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := app.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for either an error or a shutdown signal
	select {
	case err := <-errChan:
		fmt.Fprintf(os.Stderr, "Failed to start application: %v\n", err)
		os.Exit(1)
	case sig := <-sigChan:
		slog.Info("Received shutdown signal", "signal", sig)
		os.Exit(0)
	}
}
