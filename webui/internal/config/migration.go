package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// ── Legacy-only types (unexported) ───────────────────────────────────────────
// These types exist solely to read old relays.json / proxies.json files that
// were written by pre-v0.8 versions of tailrelay. They will be removed in v2.0.

type legacySocatRelay struct {
	ID         string `json:"id"`
	ListenPort int    `json:"listen_port"`
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
	Enabled    bool   `json:"enabled"`
	Autostart  bool   `json:"autostart"`
}

type legacySocatRelayList struct {
	Relays []legacySocatRelay `json:"relays"`
}

type legacyCaddyProxy struct {
	ID        string `json:"id"`
	Hostname  string `json:"hostname"`
	Port      int    `json:"port"`
	Target    string `json:"target"`
	TLS       bool   `json:"tls"`
	Enabled   bool   `json:"enabled"`
	Autostart bool   `json:"autostart"`
}

type legacyCaddyProxyList struct {
	Proxies []legacyCaddyProxy `json:"proxies"`
}

// ── MigrateFromEnvVar ─────────────────────────────────────────────────────────

// MigrateFromEnvVar migrates from the legacy RELAY_LIST environment variable
// directly to serve_relays.json. Skipped when serve_relays.json already exists.
func MigrateFromEnvVar(serveRelayConfigPath string) error {
	relayListEnv := os.Getenv("RELAY_LIST")

	// If serve_relays.json already exists, migration is done.
	if _, err := os.Stat(serveRelayConfigPath); err == nil {
		return nil
	}

	// If RELAY_LIST is empty, create empty serve_relays.json.
	if relayListEnv == "" {
		return SaveServeRelays(serveRelayConfigPath, &ServeRelayList{Relays: []ServeRelay{}})
	}

	fmt.Println("Migrating from RELAY_LIST environment variable to serve_relays.json...")
	relays, err := parseRelayListAsServe(relayListEnv)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	if err := SaveServeRelays(serveRelayConfigPath, &ServeRelayList{Relays: relays}); err != nil {
		return fmt.Errorf("failed to save serve_relays.json: %w", err)
	}

	fmt.Printf("Successfully migrated %d relays from RELAY_LIST to %s\n", len(relays), serveRelayConfigPath)
	fmt.Println("You can now remove RELAY_LIST from your environment variables")
	return nil
}

// parseRelayListAsServe parses the legacy RELAY_LIST format into ServeRelay entries.
// Format: listen_port:target_host:target_port (comma-separated)
func parseRelayListAsServe(relayList string) ([]ServeRelay, error) {
	items := strings.Split(relayList, ",")
	relays := make([]ServeRelay, 0, len(items))

	for i, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		parts := strings.Split(item, ":")
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid format for item '%s': expected listen_port:target_host:target_port", item)
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

		relays = append(relays, ServeRelay{
			ID:         fmt.Sprintf("relay-%d", i+1),
			Type:       "tcp",
			ListenPort: listenPort,
			TargetHost: targetHost,
			TargetPort: targetPort,
			Enabled:    true,
			Autostart:  true,
		})
	}

	return relays, nil
}

// ── MigrateLegacyRelaysToServe ────────────────────────────────────────────────

// MigrateLegacyRelaysToServe performs a one-shot migration from the old
// relays.json (socat) and proxies.json (caddy) formats into serve_relays.json.
// Skipped if serve_relays.json already exists.
// Will be removed in v2.0.
func MigrateLegacyRelaysToServe(paths PathsConfig) error {
	if _, err := os.Stat(paths.ServeRelayConfig); err == nil {
		fmt.Printf("serve relay migration skipped: %s already exists\n", paths.ServeRelayConfig)
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
		out.Relays = append(out.Relays, r)
	}

	// Legacy socat relays from relays.json.
	if data, err := os.ReadFile(legacySocatRelayPath(paths)); err == nil {
		var list legacySocatRelayList
		if err := json.Unmarshal(data, &list); err == nil {
			for _, r := range list.Relays {
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
	}

	// Legacy caddy proxies from proxies.json.
	if data, err := os.ReadFile(legacyCaddyProxyPath(paths)); err == nil {
		var proxies legacyCaddyProxyList
		if err := json.Unmarshal(data, &proxies); err == nil {
			for _, p := range proxies.Proxies {
				host, port := splitHostPort(p.Target)
				if host == "" || port == 0 {
					continue
				}
				addRelay(ServeRelay{
					ID:          p.ID,
					Type:        "https",
					Hostname:    p.Hostname,
					ListenPort:  p.Port,
					TargetHost:  host,
					TargetPort:  port,
					TargetHTTPS: p.TLS,
					Enabled:     p.Enabled,
					Autostart:   p.Autostart,
				})
			}
		}
	}

	return SaveServeRelays(paths.ServeRelayConfig, out)
}

// legacySocatRelayPath returns the path where old relays.json would be.
// Derive from state_dir since the config field is removed.
func legacySocatRelayPath(paths PathsConfig) string {
	if paths.StateDir != "" {
		return paths.StateDir + "/relays.json"
	}
	return "/var/lib/tailscale/relays.json"
}

// legacyCaddyProxyPath returns the path where old proxies.json would be.
func legacyCaddyProxyPath(paths PathsConfig) string {
	if paths.StateDir != "" {
		return paths.StateDir + "/proxies.json"
	}
	return "/var/lib/tailscale/proxies.json"
}

func splitHostPort(value string) (string, int) {
	v := strings.TrimSpace(value)
	v = strings.TrimPrefix(v, "http://")
	v = strings.TrimPrefix(v, "https://")
	host, portStr, err := net.SplitHostPort(v)
	if err != nil {
		return "", 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0
	}
	return host, port
}
