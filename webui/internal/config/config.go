package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from a YAML file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8021
	}
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Backup.RetentionCount == 0 {
		cfg.Backup.RetentionCount = 10
	}

	return &cfg, nil
}

// LoadOrCreate loads configuration or creates default if not exists
func LoadOrCreate(filename string) (*Config, error) {
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// Create default config
		cfg := DefaultConfig()
		if err := Save(filename, cfg); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
		return cfg, nil
	}

	return Load(filename)
}

// Save saves configuration to a YAML file
func Save(filename string, cfg *Config) error {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8021,
			Host: "0.0.0.0",
		},
		Auth: AuthConfig{
			TokenFile:           "/var/lib/tailscale/.webui_token",
			EnableTailscaleAuth: true,
			EnableTokenAuth:     true,
		},
		Paths: PathsConfig{
			CaddyConfig:      "/etc/caddy/Caddyfile",
			SocatRelayConfig: "/var/lib/tailscale/relays.json",
			CaddyProxyConfig: "/var/lib/tailscale/proxies.json",
			StateDir:         "/var/lib/tailscale",
			BackupDir:        "/var/lib/tailscale/backups",
			CertificatesDir:  "/var/lib/tailscale/certificates",
		},
		Backup: BackupConfig{
			AutoBackupEnabled:  false,
			AutoBackupSchedule: "0 2 * * *",
			RetentionCount:     10,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

// LoadSocatRelays loads socat relay configurations
func LoadSocatRelays(filename string) (*SocatRelayList, error) {
	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// Return empty list
		return &SocatRelayList{Relays: []SocatRelay{}}, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read relays file: %w", err)
	}

	var relays SocatRelayList
	if err := json.Unmarshal(data, &relays); err != nil {
		return nil, fmt.Errorf("failed to parse relays file: %w", err)
	}

	return &relays, nil
}

// SaveSocatRelays saves socat relay configurations
func SaveSocatRelays(filename string, relays *SocatRelayList) error {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(relays, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal relays: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write relays file: %w", err)
	}

	return nil
}

// GenerateToken generates a random authentication token
func GenerateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// LoadOrGenerateToken loads token from file or generates a new one
func LoadOrGenerateToken(filename string) (string, error) {
	// Check if file exists
	if _, err := os.Stat(filename); err == nil {
		data, err := os.ReadFile(filename)
		if err != nil {
			return "", fmt.Errorf("failed to read token file: %w", err)
		}
		return string(data), nil
	}

	// Generate new token
	token, err := GenerateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Save token to file
	if err := os.WriteFile(filename, []byte(token), 0600); err != nil {
		return "", fmt.Errorf("failed to save token: %w", err)
	}

	return token, nil
}
