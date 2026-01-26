package config

import "time"

// Config represents the main application configuration
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Auth    AuthConfig    `yaml:"auth"`
	Paths   PathsConfig   `yaml:"paths"`
	Backup  BackupConfig  `yaml:"backup"`
	Logging LoggingConfig `yaml:"logging"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	TokenFile           string `yaml:"token_file"`
	EnableTailscaleAuth bool   `yaml:"enable_tailscale_auth"`
	EnableTokenAuth     bool   `yaml:"enable_token_auth"`
}

// PathsConfig contains file paths for various configurations
type PathsConfig struct {
	CaddyConfig      string `yaml:"caddy_config"`
	SocatRelayConfig string `yaml:"socat_relay_config"`
	CaddyProxyConfig string `yaml:"caddy_proxy_config"`
	StateDir         string `yaml:"state_dir"`
	BackupDir        string `yaml:"backup_dir"`
	CertificatesDir  string `yaml:"certificates_dir"`
}

// BackupConfig contains backup settings
type BackupConfig struct {
	AutoBackupEnabled  bool   `yaml:"auto_backup_enabled"`
	AutoBackupSchedule string `yaml:"auto_backup_schedule"`
	RetentionCount     int    `yaml:"retention_count"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// CaddyProxy represents a Caddy reverse proxy configuration
type CaddyProxy struct {
	ID             string            `json:"id"`
	Hostname       string            `json:"hostname"`
	Port           int               `json:"port"`
	Target         string            `json:"target"`
	TLS            bool              `json:"tls"`
	TrustedProxies bool              `json:"trusted_proxies"`
	CustomHeaders  map[string]string `json:"custom_headers,omitempty"`
	Enabled        bool              `json:"enabled"`
}

// CaddyProxyList represents the list of Caddy proxies
type CaddyProxyList struct {
	Proxies []CaddyProxy `json:"proxies"`
}

// SocatRelay represents a socat TCP relay configuration
type SocatRelay struct {
	ID         string `json:"id"`
	ListenPort int    `json:"listen_port"`
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
	Enabled    bool   `json:"enabled"`
	PID        int    `json:"pid,omitempty"` // Runtime tracking
}

// SocatRelayList represents the list of socat relays
type SocatRelayList struct {
	Relays []SocatRelay `json:"relays"`
}

// BackupMetadata contains information about a backup
type BackupMetadata struct {
	Timestamp  time.Time `json:"timestamp"`
	Version    string    `json:"version"`
	Hostname   string    `json:"hostname"`
	BackupType string    `json:"backup_type"` // "full" or "config-only"
}

// BackupInfo represents information about a backup file
type BackupInfo struct {
	Filename  string         `json:"filename"`
	Size      int64          `json:"size"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  BackupMetadata `json:"metadata"`
}
