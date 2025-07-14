package app

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"cryptomarket/internal/config"
	"cryptomarket/internal/server"
)

const cfgPath = "./config/config.json"

func Start() error {
	var (
		port     = flag.Int("port", 8080, "Port number")
		helpFlag = flag.Bool("help", false, "Show help message")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  marketflow [--port <N>]\n")
		fmt.Fprintf(os.Stderr, "  marketflow --help\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  --port N     Port number\n")
	}

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0) // Exit cleanly when help is requested
	}

	slog.Info("Loading configuration...")
	config, err := config.GetConfig(cfgPath)
	if err != nil {
		slog.Error("failed to get config", "error", err)
		return fmt.Errorf("failed to get config: %w", err)
	}

	if *port > 0 {
		config.App.Port = *port
	}
	slog.Info("Configuration loaded", "port", config.App.Port)

	slog.Info("Creating application instance...")
	app := server.NewApp(config)

	slog.Info("Initializing application...")
	if err := app.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize app: %w", err)
	}

	slog.Info("Starting server...")
	app.Run()

	slog.Info("Server stopped")
	return nil
}
