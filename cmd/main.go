package main

import (
	"fmt"
	"log/slog"
	"os"

	"crypto/internal/app"
)

func main() {
	slog.Info("Starting MarketFlow application...")

	if err := app.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start application: %v\n", err)
		os.Exit(1)
	}

	select {}
}
