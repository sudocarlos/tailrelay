package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
)

func TestBackupAndRestore(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "tailrelay-backup-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Define paths
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	stateDir := filepath.Join(tempDir, "state")
	backupDir := filepath.Join(tempDir, "backups")

	// Create directories
	dirs := []string{configDir, dataDir, stateDir, backupDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	// Create dummy files
	files := map[string]string{
		filepath.Join(configDir, "Caddyfile"):         "example.com",
		filepath.Join(stateDir, "relays.json"):        `[{"id":"1"}]`,
		filepath.Join(stateDir, "proxies.json"):       `[{"id":"proxy1"}]`,
		filepath.Join(stateDir, "caddy_servers.json"): `{"server1":"1.2.3.4"}`,
		filepath.Join(stateDir, "tailscaled.state"):   "some-state-data",
		filepath.Join(stateDir, ".webui_token"):       "secret-token",
		filepath.Join(configDir, "webui.yaml"):        "server:\n  port: 8080",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", path, err)
		}
	}

	// Setup config
	cfg := &config.Config{
		ConfigFile: filepath.Join(configDir, "webui.yaml"),
		Paths: config.PathsConfig{
			CaddyConfig:      filepath.Join(configDir, "Caddyfile"),
			SocatRelayConfig: filepath.Join(stateDir, "relays.json"),
			CaddyProxyConfig: filepath.Join(stateDir, "proxies.json"),
			CaddyServerMap:   filepath.Join(stateDir, "caddy_servers.json"),
			StateDir:         stateDir,
			BackupDir:        backupDir,
			CertificatesDir:  dataDir,
		},
		Auth: config.AuthConfig{
			TokenFile: filepath.Join(stateDir, ".webui_token"),
		},
	}

	// Create manager
	manager := NewManager(cfg)

	// Create backup
	backupPath, err := manager.Create("full")
	if err != nil {
		t.Fatalf("Create backup failed: %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatalf("Backup file not created at %s", backupPath)
	}

	// Clear original files to simulate loss
	for path := range files {
		if err := os.Remove(path); err != nil {
			t.Fatalf("failed to remove original file %s: %v", path, err)
		}
	}

	// Restore backup
	if err := manager.Restore(backupPath); err != nil {
		t.Fatalf("Restore backup failed: %v", err)
	}

	// Verify files restored
	for path, expectedContent := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("failed to read restored file %s: %v", path, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("content mismatch for %s: got %s, want %s", path, string(content), expectedContent)
		}
	}
}
