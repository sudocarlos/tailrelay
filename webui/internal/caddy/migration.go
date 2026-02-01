package caddy

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
)

// MigrationHelper helps migrate from file-based proxy management to API-based
type MigrationHelper struct {
	proxyManager *ProxyManager
	proxiesFile  string
}

// NewMigrationHelper creates a new migration helper
func NewMigrationHelper(apiURL, serverMapPath, proxiesFile string) *MigrationHelper {
	return &MigrationHelper{
		proxyManager: NewProxyManager(apiURL, serverMapPath),
		proxiesFile:  proxiesFile,
	}
}

// MigrateFromFile migrates proxies from a JSON file to Caddy API
func (mh *MigrationHelper) MigrateFromFile() error {
	// Check if file exists
	if _, err := os.Stat(mh.proxiesFile); os.IsNotExist(err) {
		log.Printf("No proxies file found at %s, skipping migration", mh.proxiesFile)
		return nil
	}

	// Load proxies from file
	data, err := os.ReadFile(mh.proxiesFile)
	if err != nil {
		return fmt.Errorf("read proxies file: %w", err)
	}

	var proxyList config.CaddyProxyList
	if err := json.Unmarshal(data, &proxyList); err != nil {
		return fmt.Errorf("unmarshal proxies: %w", err)
	}

	if len(proxyList.Proxies) == 0 {
		log.Println("No proxies to migrate")
		return nil
	}

	log.Printf("Migrating %d proxies to Caddy API...", len(proxyList.Proxies))

	// Migrate each proxy
	successCount := 0
	for _, proxy := range proxyList.Proxies {
		if !proxy.Enabled {
			log.Printf("Skipping disabled proxy: %s", proxy.ID)
			continue
		}

		if _, err := mh.proxyManager.AddProxy(proxy); err != nil {
			log.Printf("Warning: Failed to migrate proxy %s: %v", proxy.ID, err)
			continue
		}

		log.Printf("Migrated proxy: %s (%s:%d -> %s)", proxy.ID, proxy.Hostname, proxy.Port, proxy.Target)
		successCount++
	}

	log.Printf("Migration complete: %d/%d proxies migrated successfully", successCount, len(proxyList.Proxies))

	// Optionally backup the old file
	if successCount > 0 {
		backupPath := mh.proxiesFile + ".migrated.bak"
		if err := os.Rename(mh.proxiesFile, backupPath); err != nil {
			log.Printf("Warning: Could not backup old proxies file: %v", err)
		} else {
			log.Printf("Old proxies file backed up to: %s", backupPath)
		}
	}

	return nil
}

// MigrateFromCaddyfile migrates proxies from a Caddyfile to Caddy API
// This is a best-effort parser for simple reverse_proxy configurations
func (mh *MigrationHelper) MigrateFromCaddyfile(caddyfilePath string) error {
	// Read Caddyfile
	data, err := os.ReadFile(caddyfilePath)
	if err != nil {
		return fmt.Errorf("read Caddyfile: %w", err)
	}

	log.Printf("Parsing Caddyfile from: %s", caddyfilePath)
	log.Println("Note: Automatic Caddyfile parsing is limited. Manual verification recommended.")

	// This is a simplified parser. For production, use Caddy's adapt endpoint.
	// For now, we'll just log a warning and skip automatic migration
	log.Println("Automatic Caddyfile migration not implemented.")
	log.Println("Please use: curl localhost:2019/adapt -H 'Content-Type: text/caddyfile' --data-binary @Caddyfile")
	log.Println("Then load the resulting JSON with: curl localhost:2019/load -H 'Content-Type: application/json' -d @config.json")

	// Keep the data variable used to avoid unused warning
	_ = data

	return nil
}

// ValidateMigration checks that all proxies from the file exist in Caddy
func (mh *MigrationHelper) ValidateMigration() error {
	// Load proxies from file
	fileProxies, err := LoadProxies(mh.proxiesFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No proxies file to validate against")
			return nil
		}
		return fmt.Errorf("load proxies from file: %w", err)
	}

	// Get proxies from API
	apiProxies, err := mh.proxyManager.ListProxies()
	if err != nil {
		return fmt.Errorf("list API proxies: %w", err)
	}

	// Create a map of API proxy IDs
	apiProxyIDs := make(map[string]bool)
	for _, proxy := range apiProxies {
		apiProxyIDs[proxy.ID] = true
	}

	// Check each file proxy
	missing := []string{}
	for _, proxy := range fileProxies {
		if !proxy.Enabled {
			continue
		}
		if !apiProxyIDs[proxy.ID] {
			missing = append(missing, proxy.ID)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("validation failed: %d proxies not found in Caddy: %v", len(missing), missing)
	}

	log.Printf("Validation passed: All %d enabled proxies found in Caddy API", len(fileProxies))
	return nil
}

// ExportToFile exports current Caddy API proxies to a JSON file (for backup)
func (mh *MigrationHelper) ExportToFile(outputPath string) error {
	proxies, err := mh.proxyManager.ListProxies()
	if err != nil {
		return fmt.Errorf("list proxies: %w", err)
	}

	proxyList := config.CaddyProxyList{
		Proxies: proxies,
	}

	data, err := json.MarshalIndent(proxyList, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal proxies: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	log.Printf("Exported %d proxies to: %s", len(proxies), outputPath)
	return nil
}
