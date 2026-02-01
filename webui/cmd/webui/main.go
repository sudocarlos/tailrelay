package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/logger"
	"github.com/sudocarlos/tailrelay-webui/internal/web"
)

//go:embed web/templates/* web/static/*
var embeddedFiles embed.FS

var (
	Version   = "v0.2.0"
	BuildTime = "dev"
)

func main() {
	// Parse command line flags
	configFile := flag.String("config", "/var/lib/tailscale/webui.yaml", "Path to configuration file")
	logLevel := flag.String("log-level", "ERROR", "Log level (DEBUG, INFO, WARN, ERROR)")
	version := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *version {
		fmt.Printf("Tailrelay Web UI %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	// Initialize logger with level from flag or environment
	level, err := logger.ParseLevel(*logLevel)
	if err != nil {
		// Try environment variable
		if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
			level, err = logger.ParseLevel(envLevel)
			if err != nil {
				level = logger.ERROR // Default to ERROR
			}
		} else {
			level = logger.ERROR
		}
	}
	logger.Init(level)
	logger.SetupStdLogger()

	logger.Info("main", "Starting Tailrelay Web UI %s (log level: %s)", Version, logger.Get().GetLevelName())

	// Load configuration
	cfg, err := config.LoadOrCreate(*configFile)
	if err != nil {
		logger.Error("main", "Failed to load configuration: %v", err)
		os.Exit(1)
	}
	logger.Info("main", "Configuration loaded from %s", *configFile)

	// Migrate from RELAY_LIST environment variable
	if err := config.MigrateFromEnvVar(cfg.Paths.SocatRelayConfig); err != nil {
		logger.Warn("main", "Migration from RELAY_LIST failed: %v", err)
	}

	// Load or generate authentication token
	authToken, err := config.LoadOrGenerateToken(cfg.Auth.TokenFile)
	if err != nil {
		logger.Error("main", "Failed to load/generate auth token: %v", err)
		os.Exit(1)
	}

	// Only display token on first run
	if _, err := os.Stat(cfg.Auth.TokenFile); os.IsNotExist(err) {
		logger.Info("main", "========================================")
		logger.Info("main", "AUTHENTICATION TOKEN (save this!): %s", authToken)
		logger.Info("main", "========================================")
	} else {
		logger.Info("main", "Using existing authentication token from %s", cfg.Auth.TokenFile)
	}

	// Get embedded filesystems
	staticFS, err := fs.Sub(embeddedFiles, "web/static")
	if err != nil {
		logger.Error("main", "Failed to load static files: %v", err)
		os.Exit(1)
	}

	templateFS, err := fs.Sub(embeddedFiles, "web/templates")
	if err != nil {
		logger.Error("main", "Failed to load templates: %v", err)
		os.Exit(1)
	}

	// Create and start web server
	server, err := web.NewServer(cfg, authToken, staticFS, templateFS)
	if err != nil {
		logger.Error("main", "Failed to create server: %v", err)
		os.Exit(1)
	}

	logger.Info("main", "Web UI available at http://0.0.0.0:%d", cfg.Server.Port)
	if cfg.Auth.EnableTailscaleAuth {
		logger.Info("main", "Tailscale network authentication: ENABLED")
	}
	if cfg.Auth.EnableTokenAuth {
		logger.Info("main", "Token authentication: ENABLED")
	}

	if err := server.Start(); err != nil {
		logger.Error("main", "Server error: %v", err)
		os.Exit(1)
	}
}
