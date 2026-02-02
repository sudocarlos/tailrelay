package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sudocarlos/tailrelay-webui/internal/caddy"
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

	// Warn once if a legacy proxy file is present; file-based configs are no longer migrated automatically.
	caddy.WarnIfLegacyProxyFile(cfg.Paths.CaddyProxyConfig)

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

	// Get filesystems (prefer disk assets for development)
	staticFS, templateFS, devDir, err := resolveWebFS()
	if err != nil {
		logger.Error("main", "Failed to load web assets: %v", err)
		os.Exit(1)
	}
	if devDir != "" {
		logger.Info("main", "Using disk UI assets from %s", devDir)
	} else {
		logger.Info("main", "Using embedded UI assets")
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

func resolveWebFS() (fs.FS, fs.FS, string, error) {
	staticFS, templateFS, devDir, err := tryDevWebFS()
	if err == nil {
		return staticFS, templateFS, devDir, nil
	}

	staticFS, err = fs.Sub(embeddedFiles, "web/static")
	if err != nil {
		return nil, nil, "", err
	}

	templateFS, err = fs.Sub(embeddedFiles, "web/templates")
	if err != nil {
		return nil, nil, "", err
	}

	return staticFS, templateFS, "", nil
}

func tryDevWebFS() (fs.FS, fs.FS, string, error) {
	devDir := os.Getenv("WEBUI_DEV_DIR")
	var candidates []string
	if devDir != "" {
		candidates = append(candidates, devDir)
	}
	// Prefer build output if present, otherwise use the source web directory.
	candidates = append(candidates, "./webui/build", "./webui/cmd/webui/web")

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}

		staticPath := filepath.Join(candidate, "static")
		templatePath := filepath.Join(candidate, "templates")

		staticInfo, statErr := os.Stat(staticPath)
		if statErr != nil || !staticInfo.IsDir() {
			continue
		}
		templateInfo, statErr := os.Stat(templatePath)
		if statErr != nil || !templateInfo.IsDir() {
			continue
		}

		return os.DirFS(staticPath), os.DirFS(templatePath), candidate, nil
	}

	return nil, nil, "", fmt.Errorf("no valid dev web assets found")
}
