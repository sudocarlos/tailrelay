package config

import (
	"encoding/json"
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

// MigrateLegacyRelaysToServe migrates legacy socat/caddy relay metadata into the
// new tailscale serve relay metadata format.
func MigrateLegacyRelaysToServe(paths PathsConfig) error {
	if _, err := os.Stat(paths.ServeRelayConfig); err == nil {
		return nil
	}

	out := &ServeRelayList{Relays: []ServeRelay{}}
	seen := map[string]struct{}{}

	addRelay := func(r ServeRelay) {
		if r.ID == "" {
			r.ID = fmt.Sprintf("%s-%d", r.Type, r.ListenPort)
		}
		key := fmt.Sprintf("%s:%d", r.Type, r.ListenPort)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		if !r.Enabled {
			r.Enabled = true
		}
		out.Relays = append(out.Relays, r)
	}

	// Legacy socat relays.
	if relays, err := LoadSocatRelays(paths.SocatRelayConfig); err == nil {
		for _, r := range relays.Relays {
			addRelay(ServeRelay{
				ID:         r.ID,
				Type:       "tcp",
				ListenPort: r.ListenPort,
				TargetHost: r.TargetHost,
				TargetPort: r.TargetPort,
				Enabled:    r.Enabled,
				Autostart:  r.Autostart,
			})
		}
	}

	// Legacy caddy proxies metadata from proxies.json.
	if data, err := os.ReadFile(paths.CaddyProxyConfig); err == nil {
		var proxies CaddyProxyList
		if err := json.Unmarshal(data, &proxies); err == nil {
			for _, p := range proxies.Proxies {
				host, port := splitHostPort(p.Target)
				if host == "" || port == 0 {
					continue
				}
				addRelay(ServeRelay{
					ID:         p.ID,
					Type:       "https",
					Hostname:   p.Hostname,
					ListenPort: p.Port,
					TargetHost: host,
					TargetPort: port,
					Enabled:    p.Enabled,
					Autostart:  p.Autostart,
				})
			}
		}
	}

	return SaveServeRelays(paths.ServeRelayConfig, out)
}

func splitHostPort(value string) (string, int) {
	v := strings.TrimSpace(value)
	v = strings.TrimPrefix(v, "http://")
	v = strings.TrimPrefix(v, "https://")
	parts := strings.Split(v, ":")
	if len(parts) < 2 {
		return "", 0
	}
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return "", 0
	}
	host := strings.Join(parts[:len(parts)-1], ":")
	return host, port
}
