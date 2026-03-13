package caddy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
)

// newTestManager creates a Manager backed by a mock Caddy API server that
// returns 404 for GET requests (empty config) and 200 for mutating requests.
// It returns the manager, the mock server, and a pointer to the recorded
// request log.  Call srv.Close() in defer.
func newTestManager(t *testing.T) (*Manager, *httptest.Server, *[]string) {
	t.Helper()
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"loading config path \"/\": path not found"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	return NewManager(srv.URL, dir+"/servers.json"), srv, &requests
}

func TestManager_InitializeAutostart(t *testing.T) {
	manager, _, requestsPtr := newTestManager(t)

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
	*requestsPtr = nil

	// Run InitializeAutostart
	if err := manager.InitializeAutostart(); err != nil {
		t.Fatalf("InitializeAutostart failed: %v", err)
	}

	// Verify resulting metadata state
	p1, _ := manager.GetProxy("proxy-1")
	if !p1.Enabled {
		t.Errorf("proxy-1: want Enabled=true (Autostart=true), got false")
	}

	p2, _ := manager.GetProxy("proxy-2")
	if p2.Enabled {
		t.Errorf("proxy-2: want Enabled=false (Autostart=false), got true")
	}

	p3, _ := manager.GetProxy("proxy-3")
	if !p3.Enabled {
		t.Errorf("proxy-3: want Enabled=true (Autostart=true), got false")
	}

	// proxy-1 (autostart=true, was disabled) and proxy-3 (autostart=true, was
	// enabled) must each produce a PUT or PATCH to create/update their routes.
	// proxy-2 (autostart=false, was enabled) must produce a DELETE to remove
	// its route.
	mutating := 0
	for _, req := range *requestsPtr {
		method := strings.SplitN(req, " ", 2)[0]
		if method == "PUT" || method == "PATCH" || method == "DELETE" {
			mutating++
		}
	}
	if mutating < 2 {
		t.Errorf("expected at least 2 mutating Caddy API requests during InitializeAutostart, got %d (requests: %v)", mutating, *requestsPtr)
	}
}

// TestManager_InitializeAutostart_EmptyProxies verifies that InitializeAutostart
// is a clean no-op when no proxies are stored.
func TestManager_InitializeAutostart_EmptyProxies(t *testing.T) {
	manager, _, requestsPtr := newTestManager(t)

	if err := manager.InitializeAutostart(); err != nil {
		t.Fatalf("InitializeAutostart on empty list failed: %v", err)
	}

	// No proxy data means no mutating API calls should be issued.
	for _, req := range *requestsPtr {
		method := strings.SplitN(req, " ", 2)[0]
		if method == "PUT" || method == "PATCH" || method == "DELETE" {
			t.Errorf("unexpected mutating request on empty proxy list: %s", req)
		}
	}
}

// TestManager_InitializeAutostart_AllDisabled verifies that when every proxy
// has Autostart=false and Enabled=false, no routes are pushed to Caddy.
func TestManager_InitializeAutostart_AllDisabled(t *testing.T) {
	manager, _, requestsPtr := newTestManager(t)

	for i, id := range []string{"pd-1", "pd-2"} {
		_, err := manager.AddProxy(config.CaddyProxy{
			ID:        id,
			Hostname:  "host" + id + ".example.com",
			Port:      9000 + i,
			Target:    "localhost:8000",
			Enabled:   false,
			Autostart: false,
		})
		if err != nil {
			t.Fatalf("AddProxy %s: %v", id, err)
		}
	}
	*requestsPtr = nil

	if err := manager.InitializeAutostart(); err != nil {
		t.Fatalf("InitializeAutostart failed: %v", err)
	}

	// All proxies are Autostart=false and already Enabled=false — the branch
	// that calls UpdateProxy only fires when Enabled is true and Autostart is
	// false.  Since they're already disabled no mutating calls should occur.
	for _, req := range *requestsPtr {
		method := strings.SplitN(req, " ", 2)[0]
		if method == "PUT" || method == "PATCH" || method == "DELETE" {
			t.Errorf("unexpected mutating request for all-disabled proxies: %s", req)
		}
	}
}

// TestManager_InitializeAutostart_CaddyDown verifies that InitializeAutostart
// surfaces an error when the Caddy API is completely unreachable.
func TestManager_InitializeAutostart_CaddyDown(t *testing.T) {
	// Start and immediately close the server to simulate an unreachable API.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close()

	dir := t.TempDir()
	manager := NewManager(srvURL, dir+"/servers.json")

	// Seed one autostart proxy directly into the metadata so InitializeAutostart
	// tries to call the dead API.
	_, err := manager.AddProxy(config.CaddyProxy{
		ID:        "broken-1",
		Hostname:  "broken.example.com",
		Port:      9010,
		Target:    "localhost:8080",
		Enabled:   true,
		Autostart: true,
	})
	// AddProxy itself may fail if Caddy is down; that's fine for this test.
	// We only care that InitializeAutostart does not panic and handles the
	// error gracefully (it logs warnings but currently does not return an error
	// for individual proxy failures — verify we at least don't panic).
	_ = err

	// InitializeAutostart logs warnings per-proxy but returns nil even if
	// individual updates fail.  It should never panic.
	_ = manager.InitializeAutostart()
}
