package caddy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
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

// jsonRoundTrip serialises route to JSON and back, converting all nested typed
// structs to map[string]interface{} — exactly the shape produced by reading
// from the Caddy Admin API. This is required before calling extractReverseProxyHandler
// or routeToProxy, which rely on that shape.
func jsonRoundTrip(t *testing.T, route *Route) Route {
	t.Helper()
	b, err := json.Marshal(route)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var out Route
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return out
}

// TestBuildRoute_TLSTransport verifies that buildRoute emits the correct Caddy
// transport block for each of the three HTTPS target modes:
//   - off:      no transport block
//   - insecure: transport with insecure_skip_verify=true
//   - cert:     transport with ca.pem_files set (TLS flag stays false)
func TestBuildRoute_TLSTransport(t *testing.T) {
	pm := newTestProxyManager(t, "http://127.0.0.1:1") // URL never called

	tests := []struct {
		name          string
		proxy         config.CaddyProxy
		wantTransport bool
		wantInsecure  bool
		wantCAFile    string
	}{
		{
			name:          "http target — no transport block",
			proxy:         config.CaddyProxy{ID: "p1", Hostname: "host.example.com", Target: "backend:8080"},
			wantTransport: false,
		},
		{
			name:          "insecure HTTPS — insecure_skip_verify transport",
			proxy:         config.CaddyProxy{ID: "p2", Hostname: "host.example.com", Target: "backend:8443", TLS: true},
			wantTransport: true,
			wantInsecure:  true,
		},
		{
			name:          "custom CA cert — ca transport, TLS flag false",
			proxy:         config.CaddyProxy{ID: "p3", Hostname: "host.example.com", Target: "backend:8443", TLSCertFile: "/certs/ca.pem"},
			wantTransport: true,
			wantInsecure:  false,
			wantCAFile:    "/certs/ca.pem",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			route, err := pm.buildRoute(tc.proxy)
			if err != nil {
				t.Fatalf("buildRoute() error: %v", err)
			}

			// JSON round-trip converts typed structs to map[string]interface{},
			// matching what extractReverseProxyHandler expects.
			rt := jsonRoundTrip(t, route)
			rpHandler, ok := extractReverseProxyHandler(rt)
			if !ok {
				t.Fatal("extractReverseProxyHandler: not found")
			}

			transport, hasTransport := rpHandler["transport"]
			if tc.wantTransport != hasTransport {
				t.Fatalf("wantTransport=%v, got hasTransport=%v", tc.wantTransport, hasTransport)
			}
			if !tc.wantTransport {
				return
			}

			tMap, ok := transport.(map[string]interface{})
			if !ok {
				t.Fatalf("transport is not a map, got %T", transport)
			}
			tlsRaw, hasTLS := tMap["tls"]
			if !hasTLS {
				t.Fatal("transport has no tls key")
			}
			tlsMap, ok := tlsRaw.(map[string]interface{})
			if !ok {
				t.Fatalf("transport.tls is not a map, got %T", tlsRaw)
			}

			gotInsecure, _ := tlsMap["insecure_skip_verify"].(bool)
			if tc.wantInsecure && !gotInsecure {
				t.Error("want insecure_skip_verify=true, got false")
			}
			if !tc.wantInsecure && gotInsecure {
				t.Error("want insecure_skip_verify=false, got true")
			}

			caCfg, hasCA := tlsMap["ca"].(map[string]interface{})
			if tc.wantCAFile != "" {
				if !hasCA {
					t.Fatal("want ca config, got none")
				}
				pemFiles, _ := caCfg["pem_files"].([]interface{})
				if len(pemFiles) == 0 {
					t.Fatal("want pem_files entry, got none")
				}
				if got, _ := pemFiles[0].(string); got != tc.wantCAFile {
					t.Errorf("want CA PEM file %q, got %q", tc.wantCAFile, got)
				}
			} else if hasCA {
				t.Errorf("want no ca config, got %+v", caCfg)
			}
		})
	}
}

// TestRouteToProxy_TLSRoundTrip verifies that routeToProxy correctly restores
// proxy.TLS and proxy.TLSCertFile from the Caddy route — specifically that:
//   - insecure_skip_verify=true  → proxy.TLS=true, proxy.TLSCertFile=""
//   - ca.pem_files set           → proxy.TLSCertFile set, proxy.TLS=false
//   - no transport               → both false/empty
func TestRouteToProxy_TLSRoundTrip(t *testing.T) {
	pm := newTestProxyManager(t, "http://127.0.0.1:1")

	tests := []struct {
		name            string
		proxy           config.CaddyProxy
		wantTLS         bool
		wantTLSCertFile string
	}{
		{
			name:  "no transport — both false",
			proxy: config.CaddyProxy{ID: "p1", Hostname: "host.example.com", Target: "backend:8080"},
		},
		{
			name:    "insecure — TLS=true restored",
			proxy:   config.CaddyProxy{ID: "p2", Hostname: "host.example.com", Target: "backend:8443", TLS: true},
			wantTLS: true,
		},
		{
			name:            "custom CA — TLSCertFile restored, TLS=false",
			proxy:           config.CaddyProxy{ID: "p3", Hostname: "host.example.com", Target: "backend:8443", TLSCertFile: "/certs/ca.pem"},
			wantTLSCertFile: "/certs/ca.pem",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			route, err := pm.buildRoute(tc.proxy)
			if err != nil {
				t.Fatalf("buildRoute() error: %v", err)
			}

			// Simulate Caddy API round-trip before calling routeToProxy.
			rt := jsonRoundTrip(t, route)
			restored, err := pm.routeToProxy(rt)
			if err != nil {
				t.Fatalf("routeToProxy() error: %v", err)
			}

			if restored.TLS != tc.wantTLS {
				t.Errorf("TLS: want %v, got %v", tc.wantTLS, restored.TLS)
			}
			if restored.TLSCertFile != tc.wantTLSCertFile {
				t.Errorf("TLSCertFile: want %q, got %q", tc.wantTLSCertFile, restored.TLSCertFile)
			}
		})
	}
}
