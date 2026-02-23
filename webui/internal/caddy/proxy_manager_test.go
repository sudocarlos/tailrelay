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
// (not an error) when Caddy's config is {} and the /apps/http/servers path returns 404.
func TestListServers_EmptyCaddyConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"loading config path \"/apps/http/servers\": path not found"}`))
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
		w.Write([]byte(`{"error":"path not found"}`))
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
