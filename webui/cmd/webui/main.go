package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
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
	version := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *version {
		fmt.Printf("Tailrelay Web UI %s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	log.Printf("Starting Tailrelay Web UI %s", Version)

	// Load configuration
	cfg, err := config.LoadOrCreate(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	log.Printf("Configuration loaded from %s", *configFile)

	// Migrate from RELAY_LIST environment variable
	if err := config.MigrateFromEnvVar(cfg.Paths.SocatRelayConfig); err != nil {
		log.Printf("Warning: Migration from RELAY_LIST failed: %v", err)
	}

	// Load or generate authentication token
	authToken, err := config.LoadOrGenerateToken(cfg.Auth.TokenFile)
	if err != nil {
		log.Fatalf("Failed to load/generate auth token: %v", err)
	}

	// Only display token on first run
	if _, err := os.Stat(cfg.Auth.TokenFile); os.IsNotExist(err) {
		log.Printf("========================================")
		log.Printf("AUTHENTICATION TOKEN (save this!): %s", authToken)
		log.Printf("========================================")
	} else {
		log.Printf("Using existing authentication token from %s", cfg.Auth.TokenFile)
	}

	// Get embedded filesystems
	staticFS, err := fs.Sub(embeddedFiles, "web/static")
	if err != nil {
		log.Fatalf("Failed to load static files: %v", err)
	}

	templateFS, err := fs.Sub(embeddedFiles, "web/templates")
	if err != nil {
		log.Fatalf("Failed to load templates: %v", err)
	}

	// Create and start web server
	server, err := web.NewServer(cfg, authToken, staticFS, templateFS)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("Web UI available at http://0.0.0.0:%d", cfg.Server.Port)
	if cfg.Auth.EnableTailscaleAuth {
		log.Printf("Tailscale network authentication: ENABLED")
	}
	if cfg.Auth.EnableTokenAuth {
		log.Printf("Token authentication: ENABLED")
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
