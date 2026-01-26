package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MigrateFromEnvVar migrates from RELAY_LIST environment variable to relays.json
func MigrateFromEnvVar(relaysConfigPath string) error {
	relayListEnv := os.Getenv("RELAY_LIST")

	// If relays.json exists, migration already done
	if _, err := os.Stat(relaysConfigPath); err == nil {
		return nil
	}

	// If RELAY_LIST is empty, create empty relays.json
	if relayListEnv == "" {
		emptyList := &SocatRelayList{Relays: []SocatRelay{}}
		return SaveSocatRelays(relaysConfigPath, emptyList)
	}

	// Parse RELAY_LIST and create relays.json
	fmt.Println("Migrating from RELAY_LIST environment variable...")
	relays, err := parseRelayList(relayListEnv)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Save to relays.json
	relayList := &SocatRelayList{Relays: relays}
	if err := SaveSocatRelays(relaysConfigPath, relayList); err != nil {
		return fmt.Errorf("failed to save relays.json: %w", err)
	}

	fmt.Printf("Successfully migrated %d relays to %s\n", len(relays), relaysConfigPath)
	fmt.Println("You can now remove RELAY_LIST from your environment variables")
	return nil
}

// parseRelayList parses the RELAY_LIST environment variable format
// Format: port:host:port,port:host:port
func parseRelayList(relayList string) ([]SocatRelay, error) {
	items := strings.Split(relayList, ",")
	relays := make([]SocatRelay, 0, len(items))

	for i, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		parts := strings.Split(item, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid format for item '%s': expected format is 'port:host:port'", item)
		}

		listenPort, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("invalid listen port '%s': %w", parts[0], err)
		}

		targetHost := parts[1]
		if targetHost == "" {
			return nil, fmt.Errorf("target host cannot be empty in item '%s'", item)
		}

		targetPort, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid target port '%s': %w", parts[2], err)
		}

		relay := SocatRelay{
			ID:         fmt.Sprintf("relay-%d", i+1),
			ListenPort: listenPort,
			TargetHost: targetHost,
			TargetPort: targetPort,
			Enabled:    true,
		}
		relays = append(relays, relay)
	}

	return relays, nil
}
