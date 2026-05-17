package backup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
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
		filepath.Join(stateDir, "serve_relays.json"): `[]`,
		filepath.Join(stateDir, "tailscaled.state"):  "some-state-data",
		filepath.Join(stateDir, ".webui_token"):      "secret-token",
		filepath.Join(configDir, "webui.yaml"):       "server:\n  port: 8080",
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
			ServeRelayConfig: filepath.Join(stateDir, "serve_relays.json"),
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
		t.Fatalf("Restore backup failed: %v\n(Note: files in dummy map that are no longer backed up will not be restored)", err)
	}

	// Verify files restored (only those still backed up)
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
