package main

import (
	"fmt"
	"log"

	"github.com/sudocarlos/tailrelay-webui/internal/caddy"
	"github.com/sudocarlos/tailrelay-webui/internal/config"
)

// Example usage of the new Caddy API integration
func main() {
	// Create a Caddy manager
	manager := caddy.NewManager(
		caddy.DefaultAdminAPI, // http://localhost:2019
		"tailrelay",           // Server name in Caddy config
	)

	// Example 1: Check Caddy status
	fmt.Println("=== Checking Caddy Status ===")
	running, err := manager.GetStatus()
	if err != nil {
		log.Fatalf("Failed to check Caddy status: %v", err)
	}
	fmt.Printf("Caddy API accessible: %v\n\n", running)

	// Example 2: Initialize server (one-time setup)
	fmt.Println("=== Initializing Server ===")
	err = manager.InitializeServer([]string{":80", ":443"})
	if err != nil {
		log.Printf("Server initialization: %v (may already exist)\n", err)
	} else {
		fmt.Println("Server initialized successfully")
	}
	fmt.Println()

	// Example 3: Add a new proxy
	fmt.Println("=== Adding New Proxy ===")
	proxy := config.CaddyProxy{
		ID:       "example-proxy",
		Hostname: "myserver.tailnet.ts.net",
		Port:     8080,
		Target:   "localhost:9000",
		Enabled:  true,
	}
	err = manager.AddProxy(proxy)
	if err != nil {
		log.Printf("Failed to add proxy: %v\n", err)
	} else {
		fmt.Printf("Proxy added: %s\n", proxy.ID)
	}
	fmt.Println()

	// Example 4: List all proxies
	fmt.Println("=== Listing All Proxies ===")
	proxies, err := manager.ListProxies()
	if err != nil {
		log.Fatalf("Failed to list proxies: %v", err)
	}
	fmt.Printf("Found %d proxies:\n", len(proxies))
	for _, p := range proxies {
		fmt.Printf("  - %s: %s:%d -> %s (enabled: %v)\n",
			p.ID, p.Hostname, p.Port, p.Target, p.Enabled)
	}
	fmt.Println()

	// Example 5: Get a specific proxy
	fmt.Println("=== Getting Specific Proxy ===")
	retrievedProxy, err := manager.GetProxy("example-proxy")
	if err != nil {
		log.Printf("Failed to get proxy: %v\n", err)
	} else {
		fmt.Printf("Retrieved: %s -> %s\n", retrievedProxy.Hostname, retrievedProxy.Target)
	}
	fmt.Println()

	// Example 6: Update proxy
	fmt.Println("=== Updating Proxy ===")
	if retrievedProxy != nil {
		retrievedProxy.Target = "localhost:9001"
		err = manager.UpdateProxy(*retrievedProxy)
		if err != nil {
			log.Printf("Failed to update proxy: %v\n", err)
		} else {
			fmt.Printf("Proxy updated: new target = %s\n", retrievedProxy.Target)
		}
	}
	fmt.Println()

	// Example 7: Toggle proxy (disable)
	fmt.Println("=== Toggling Proxy ===")
	err = manager.ToggleProxy("example-proxy", false)
	if err != nil {
		log.Printf("Failed to toggle proxy: %v\n", err)
	} else {
		fmt.Println("Proxy disabled")
	}
	fmt.Println()

	// Example 8: Get upstream status
	fmt.Println("=== Getting Upstream Status ===")
	upstreams, err := manager.GetUpstreams()
	if err != nil {
		log.Printf("Failed to get upstreams: %v\n", err)
	} else {
		fmt.Printf("Found %d upstreams:\n", len(upstreams))
		for _, u := range upstreams {
			fmt.Printf("  - %s: requests=%d, fails=%d\n",
				u.Address, u.NumRequests, u.Fails)
		}
	}
	fmt.Println()

	// Example 9: Delete proxy
	fmt.Println("=== Deleting Proxy ===")
	err = manager.DeleteProxy("example-proxy")
	if err != nil {
		log.Printf("Failed to delete proxy: %v\n", err)
	} else {
		fmt.Println("Proxy deleted successfully")
	}
	fmt.Println()

	// Example 10: Add HTTPS proxy with TLS
	fmt.Println("=== Adding HTTPS Proxy ===")
	httpsProxy := config.CaddyProxy{
		ID:       "secure-proxy",
		Hostname: "secure.tailnet.ts.net",
		Port:     8443,
		Target:   "https://backend.local:8443",
		TLS:      true,
		Enabled:  true,
		CustomHeaders: map[string]string{
			"X-TLS-Cert": "/var/lib/tailscale/tls.cert",
		},
	}
	err = manager.AddProxy(httpsProxy)
	if err != nil {
		log.Printf("Failed to add HTTPS proxy: %v\n", err)
	} else {
		fmt.Printf("HTTPS proxy added: %s\n", httpsProxy.ID)
	}
	fmt.Println()

	// Cleanup
	fmt.Println("=== Cleanup ===")
	manager.DeleteProxy("secure-proxy")
	fmt.Println("Example completed!")
}
