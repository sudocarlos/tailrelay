package caddy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestProxyManager creates a ProxyManager pointed at the given test server URL,
// using a temporary directory for server map and metadata storage.
func newTestProxyManager(t *testing.T, apiURL string) *ProxyManager {
	t.Helper()
	dir := t.TempDir()
	return NewProxyManager(apiURL, dir+"/servers.json")
}

// TestListServers_EmptyCaddyConfig verifies that listServers returns an empty map
// (not an error) when Caddy's config is {} and the root path returns 404.
func TestListServers_EmptyCaddyConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"loading config path \"/\": path not found"}`))
	}))
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)
	servers, err := pm.listServers()
	if err != nil {
		t.Fatalf("listServers() returned unexpected error for empty Caddy config: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected empty server map, got %d entries", len(servers))
	}
}

// TestListServers_NullResponse verifies that listServers returns an empty map
// when Caddy returns a null JSON value for the servers path.
func TestListServers_NullResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`null`))
	}))
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)
	servers, err := pm.listServers()
	if err != nil {
		t.Fatalf("listServers() returned unexpected error for null response: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected empty server map, got %d entries", len(servers))
	}
}

// TestListServers_EmptyObjectResponse verifies that listServers returns an empty map
// when Caddy returns {} for the servers path.
func TestListServers_EmptyObjectResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)
	servers, err := pm.listServers()
	if err != nil {
		t.Fatalf("listServers() returned unexpected error for empty object response: %v", err)
	}
	if len(servers) != 0 {
		t.Fatalf("expected empty server map, got %d entries", len(servers))
	}
}

// TestAllocateServerName_EmptyCaddyConfig verifies that allocateServerName succeeds
// and returns "srv0" when the Caddy config is empty (listServers returns empty map).
func TestAllocateServerName_EmptyCaddyConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"loading config path \"/\": path not found"}`))
	}))
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)
	name, err := pm.allocateServerName()
	if err != nil {
		t.Fatalf("allocateServerName() returned unexpected error: %v", err)
	}
	if name != "srv0" {
		t.Fatalf("expected server name srv0, got %s", name)
	}
}

// TestListServers_RealError verifies that non-404 errors are still propagated.
func TestListServers_RealError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)
	_, err := pm.listServers()
	if err == nil {
		t.Fatal("listServers() expected an error for 500 response, got nil")
	}
}

// TestAllocateServerName_WithExistingServers verifies that allocateServerName
// skips names already in use by Caddy and returns the next available one.
func TestAllocateServerName_WithExistingServers(t *testing.T) {
	// Mock Caddy that reports srv0 and srv1 as already existing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"apps": {
				"http": {
					"servers": {
						"srv0": {"listen": [":8080"]},
						"srv1": {"listen": [":8081"]}
					}
				}
			}
		}`))
	}))
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)
	name, err := pm.allocateServerName()
	if err != nil {
		t.Fatalf("allocateServerName() unexpected error: %v", err)
	}
	if name != "srv2" {
		t.Fatalf("expected srv2 (srv0 and srv1 taken), got %s", name)
	}
}

// TestListServers_PopulatedResponse verifies that listServers correctly parses
// a response containing real server entries.
func TestListServers_PopulatedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"apps": {
				"http": {
					"servers": {
						"srv0": {"listen": [":9000"]},
						"srv1": {"listen": [":9001"]}
					}
				}
			}
		}`))
	}))
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)
	servers, err := pm.listServers()
	if err != nil {
		t.Fatalf("listServers() unexpected error: %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
	if _, ok := servers["srv0"]; !ok {
		t.Error("expected srv0 in server map")
	}
	if _, ok := servers["srv1"]; !ok {
		t.Error("expected srv1 in server map")
	}
}

// TestListServers_MalformedJSON verifies that listServers returns an error
// (rather than panicking) when the Caddy API responds with invalid JSON.
func TestListServers_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{this is not valid json`))
	}))
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)
	_, err := pm.listServers()
	if err == nil {
		t.Fatal("listServers() expected an error for malformed JSON, got nil")
	}
}
