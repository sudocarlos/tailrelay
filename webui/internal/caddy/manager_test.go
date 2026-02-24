package caddy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
)

func TestManager_InitializeAutostart(t *testing.T) {
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqStr := r.Method + " " + r.URL.Path
		requests = append(requests, reqStr)
		// Return 404 for listServers and ensureHTTPServersPath to mimic empty Caddy
		if r.Method == http.MethodGet && (r.URL.Path == "/config/apps/http/servers" || r.URL.Path == "/config" || r.URL.Path == "/config/") {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"loading config path \"/\": path not found"}`))
			return
		}

		// For everything else (PUT, PATCH, DELETE), return OK
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`)) // provide a valid json response body
	}))
	defer srv.Close()

	dir := t.TempDir()
	serverMapPath := dir + "/servers.json"

	manager := NewManager(srv.URL, serverMapPath)

	// Add proxy 1: Autostart true, initially Enabled false
	_, err := manager.AddProxy(config.CaddyProxy{
		ID:        "proxy-1",
		Hostname:  "test1.com",
		Port:      8081,
		Target:    "localhost:9091",
		Enabled:   false,
		Autostart: true,
	})
	if err != nil {
		t.Fatalf("Failed to add proxy-1: %v", err)
	}

	// Add proxy 2: Autostart false, initially Enabled true
	_, err = manager.AddProxy(config.CaddyProxy{
		ID:        "proxy-2",
		Hostname:  "test2.com",
		Port:      8082,
		Target:    "localhost:9092",
		Enabled:   true,
		Autostart: false,
	})
	if err != nil {
		t.Fatalf("Failed to add proxy-2: %v", err)
	}

	// Add proxy 3: Autostart true, initially Enabled true
	_, err = manager.AddProxy(config.CaddyProxy{
		ID:        "proxy-3",
		Hostname:  "test3.com",
		Port:      8083,
		Target:    "localhost:9093",
		Enabled:   true,
		Autostart: true,
	})
	if err != nil {
		t.Fatalf("Failed to add proxy-3: %v", err)
	}

	// Clear request log from the initial AddProxy calls
	requests = []string{}

	// Run InitializeAutostart
	if err := manager.InitializeAutostart(); err != nil {
		t.Fatalf("InitializeAutostart failed: %v", err)
	}

	// Verify resulting metadata state
	p1, _ := manager.GetProxy("proxy-1")
	if !p1.Enabled {
		t.Errorf("Proxy 1 should be enabled because Autostart is true")
	}

	p2, _ := manager.GetProxy("proxy-2")
	if p2.Enabled {
		t.Errorf("Proxy 2 should be disabled because Autostart is false, overriding its previous state")
	}

	p3, _ := manager.GetProxy("proxy-3")
	if !p3.Enabled {
		t.Errorf("Proxy 3 should be enabled because Autostart is true")
	}

	// Verify that the mock server received the expected requests
	t.Logf("Recorded requests: %v", requests)
	if len(requests) == 0 {
		t.Errorf("Expected Caddy API requests during InitializeAutostart, but got none")
	}

	// proxy-1 and proxy-3 should result in PUT or PATCH (create or update route) requests
	updateReqs := 0
	for _, req := range requests {
		if (len(req) > 3) && (req[:3] == "PUT" || req[:5] == "PATCH") && len(req) > 30 && req[len(req)-4:] != "fig/" {
			// Count requests like "PUT /config/apps/http/servers/srv..." or "PATCH /config/apps/http/servers/srv..."
			// But exclude "PATCH /config/"
			updateReqs++
		}
	}

	if updateReqs < 2 {
		t.Errorf("Expected at least 2 PUT/PATCH requests for autostart proxies, got %d", updateReqs)
	}
}
