package config

import "time"

// Config represents the main application configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Auth      AuthConfig      `yaml:"auth"`
	Paths     PathsConfig     `yaml:"paths"`
	Backup    BackupConfig    `yaml:"backup"`
	Logging   LoggingConfig   `yaml:"logging"`
	Tailscale TailscaleConfig `yaml:"tailscale"`
	// Internal fields
	ConfigFile string `yaml:"-"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	TokenFile     string `yaml:"token_file"`
	AdminHashFile string `yaml:"admin_hash_file"`
}

// PathsConfig contains file paths for various configurations
type PathsConfig struct {
	ServeRelayConfig string `yaml:"serve_relay_config"`
	StateDir         string `yaml:"state_dir"`
	BackupDir        string `yaml:"backup_dir"`
	CertificatesDir  string `yaml:"certificates_dir"`
	TargetsFile      string `yaml:"targets_file"`
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

// TailscaleConfig contains persisted Tailscale login settings.
type TailscaleConfig struct {
	// ControlServer is a custom control server URL (e.g. a self-hosted
	// Headscale instance) passed as `tailscale login --login-server=<url>`.
	// Empty means Tailscale's default control plane.
	ControlServer string `yaml:"control_server"`
}

// ServeRelay represents a relay backed by `tailscale serve` or `tailscale funnel`.
type ServeRelay struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // "https", "tcp", or "funnel"
	Hostname    string `json:"hostname,omitempty"`
	ListenPort  int    `json:"listen_port"`
	TargetHost  string `json:"target_host"`
	TargetPort  int    `json:"target_port"`
	TargetHTTPS bool   `json:"target_https"`
	Enabled     bool   `json:"enabled"`
	Autostart   bool   `json:"autostart"`
	// FunnelTransport selects the underlying `tailscale funnel` transport
	// ("https" or "tcp") when Type is "funnel". Ignored otherwise.
	FunnelTransport string `json:"funnel_transport,omitempty"`
}

// ServeRelayList represents the list of tailscale serve relay definitions.
type ServeRelayList struct {
	Relays []ServeRelay `json:"relays"`
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
