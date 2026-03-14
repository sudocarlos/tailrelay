package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/caddy"
	"github.com/sudocarlos/tailrelay/internal/config"
)

// newTestCaddyHandler creates a CaddyHandler backed by a mock Caddy API server.
// The mock returns 404 for GET requests (no existing config) and 200 `{}` for
// all mutating requests. tsReady is always true so cert probing is enabled but
// calls will safely fail (no real Caddy/Tailscale present).
func newTestCaddyHandler(t *testing.T) (*CaddyHandler, *[]string) {
	t.Helper()
	var requests []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"path not found"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			CaddyServerMap:  filepath.Join(dir, "servers.json"),
			CertificatesDir: filepath.Join(dir, "certs"),
		},
	}
	manager := caddy.NewManager(srv.URL, cfg.Paths.CaddyServerMap)
	h := NewCaddyHandlerWithManager(cfg, nil, manager, func() bool { return true })
	return h, &requests
}

// jsonBody encodes v as JSON and returns a *bytes.Buffer.
func jsonBody(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatalf("jsonBody encode: %v", err)
	}
	return &buf
}

// --- APIList ---

func TestCaddyHandler_APIList_Empty(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/caddy", nil)
	rr := httptest.NewRecorder()
	h.APIList(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected application/json content-type, got %q", rr.Header().Get("Content-Type"))
	}
	// Should decode as a JSON array (empty or not).
	var result []interface{}
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("response is not a JSON array: %v", err)
	}
}

func TestCaddyHandler_APIList_WithProxy(t *testing.T) {
	h, _ := newTestCaddyHandler(t)

	// Seed a proxy via Create.
	body := jsonBody(t, config.CaddyProxy{
		ID:       "p1",
		Hostname: "example.ts.net",
		Port:     8081,
		Target:   "http://localhost:9091",
		Enabled:  true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/create", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("Create: expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/caddy", nil)
	rr2 := httptest.NewRecorder()
	h.APIList(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("APIList after Create: expected 200, got %d", rr2.Code)
	}
	var list []map[string]interface{}
	if err := json.NewDecoder(rr2.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 proxy, got %d", len(list))
	}
}

// --- APIGet ---

func TestCaddyHandler_APIGet_MissingID(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/caddy/get", nil)
	rr := httptest.NewRecorder()
	h.APIGet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing id, got %d", rr.Code)
	}
}

func TestCaddyHandler_APIGet_NotFound(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/caddy/get?id=nonexistent", nil)
	rr := httptest.NewRecorder()
	h.APIGet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown proxy, got %d", rr.Code)
	}
}

func TestCaddyHandler_APIGet_Found(t *testing.T) {
	h, _ := newTestCaddyHandler(t)

	// Create a proxy first.
	body := jsonBody(t, config.CaddyProxy{
		ID:      "get-me",
		Port:    8082,
		Target:  "http://localhost:9082",
		Enabled: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/create", body)
	req.Header.Set("Content-Type", "application/json")
	httptest.NewRecorder() // discard
	h.Create(httptest.NewRecorder(), req)

	req2 := httptest.NewRequest(http.MethodGet, "/api/caddy/get?id=get-me", nil)
	rr := httptest.NewRecorder()
	h.APIGet(rr, req2)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var proxy config.CaddyProxy
	if err := json.NewDecoder(rr.Body).Decode(&proxy); err != nil {
		t.Fatalf("decode proxy: %v", err)
	}
	if proxy.ID != "get-me" {
		t.Errorf("expected proxy id 'get-me', got %q", proxy.ID)
	}
}

// --- Create ---

func TestCaddyHandler_Create_WrongMethod(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/caddy/create", nil)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestCaddyHandler_Create_InvalidBody(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/create", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", rr.Code)
	}
}

func TestCaddyHandler_Create_Success(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	body := jsonBody(t, config.CaddyProxy{
		Hostname: "new.ts.net",
		Port:     8090,
		Target:   "http://localhost:9090",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/create", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status=success, got %v", resp["status"])
	}
	if resp["proxy"] == nil {
		t.Error("expected proxy object in response")
	}
}

func TestCaddyHandler_Create_BareHostPort(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	body := jsonBody(t, config.CaddyProxy{
		Hostname: "new.ts.net",
		Port:     8090,
		Target:   "mempool.embassy:8080",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/create", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bare host:port target, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "http or https") {
		t.Errorf("expected error mentioning 'http or https', got %q", rr.Body.String())
	}
}

func TestCaddyHandler_Create_EmptyTarget(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	body := jsonBody(t, config.CaddyProxy{
		Hostname: "new.ts.net",
		Port:     8090,
		Target:   "",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/create", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty target, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "target is required") {
		t.Errorf("expected error mentioning 'target is required', got %q", rr.Body.String())
	}
}

func TestCaddyHandler_Create_HttpsTarget(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	body := jsonBody(t, config.CaddyProxy{
		Hostname: "new.ts.net",
		Port:     8090,
		Target:   "https://mempool.embassy:8080",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/create", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for https:// target, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status=success, got %v", resp["status"])
	}
}

// --- Update ---

func TestCaddyHandler_Update_WrongMethod(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/caddy/update", nil)
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestCaddyHandler_Update_MissingID(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	body := jsonBody(t, config.CaddyProxy{Port: 8080, Target: "http://localhost:9080"})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/update", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing id, got %d", rr.Code)
	}
}

func TestCaddyHandler_Update_BareHostPort(t *testing.T) {
	h, _ := newTestCaddyHandler(t)

	// Create a valid proxy first.
	createBody := jsonBody(t, config.CaddyProxy{
		ID:      "upd-scheme",
		Port:    8095,
		Target:  "http://localhost:9095",
		Enabled: true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/caddy/create", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), createReq)

	// Attempt update with bare host:port target.
	updateBody := jsonBody(t, config.CaddyProxy{
		ID:      "upd-scheme",
		Port:    8095,
		Target:  "mempool.embassy:8080",
		Enabled: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/update", updateBody)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bare host:port target on update, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "http or https") {
		t.Errorf("expected error mentioning 'http or https', got %q", rr.Body.String())
	}
}

func TestCaddyHandler_Update_Success(t *testing.T) {
	h, _ := newTestCaddyHandler(t)

	// Create a proxy to update.
	createBody := jsonBody(t, config.CaddyProxy{
		ID:      "upd-1",
		Port:    8091,
		Target:  "http://localhost:9091",
		Enabled: true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/caddy/create", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), createReq)

	// Now update it.
	updateBody := jsonBody(t, config.CaddyProxy{
		ID:      "upd-1",
		Port:    8091,
		Target:  "http://localhost:9999",
		Enabled: true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/update", updateBody)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status=success, got %v", resp["status"])
	}
}

// --- Delete ---

func TestCaddyHandler_Delete_WrongMethod(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/caddy/delete", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestCaddyHandler_Delete_MissingID(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/delete", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing id, got %d", rr.Code)
	}
}

func TestCaddyHandler_Delete_Success(t *testing.T) {
	h, _ := newTestCaddyHandler(t)

	// Create then delete.
	createBody := jsonBody(t, config.CaddyProxy{
		ID:      "del-1",
		Port:    8092,
		Target:  "http://localhost:9092",
		Enabled: true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/caddy/create", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), createReq)

	req := httptest.NewRequest(http.MethodPost, "/api/caddy/delete?id=del-1", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status=success, got %v", resp["status"])
	}
}

// --- Toggle ---

func TestCaddyHandler_Toggle_WrongMethod(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/caddy/toggle", nil)
	rr := httptest.NewRecorder()
	h.Toggle(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestCaddyHandler_Toggle_MissingID(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	body := jsonBody(t, map[string]interface{}{"id": "", "enabled": true})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/toggle", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Toggle(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing id, got %d", rr.Code)
	}
}

func TestCaddyHandler_Toggle_Success(t *testing.T) {
	h, _ := newTestCaddyHandler(t)

	// Create a proxy to toggle.
	createBody := jsonBody(t, config.CaddyProxy{
		ID:      "tog-1",
		Port:    8093,
		Target:  "http://localhost:9093",
		Enabled: true,
	})
	createReq := httptest.NewRequest(http.MethodPost, "/api/caddy/create", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	h.Create(httptest.NewRecorder(), createReq)

	body := jsonBody(t, map[string]interface{}{"id": "tog-1", "enabled": false})
	req := httptest.NewRequest(http.MethodPost, "/api/caddy/toggle", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Toggle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status=success, got %v", resp["status"])
	}
}

// --- Reload ---

func TestCaddyHandler_Reload_WrongMethod(t *testing.T) {
	h, _ := newTestCaddyHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/caddy/reload", nil)
	rr := httptest.NewRecorder()
	h.Reload(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestCaddyHandler_Reload_CaddyDown(t *testing.T) {
	// Point manager at a server that's already closed to simulate Caddy being down.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srvURL := srv.URL
	srv.Close()

	dir := t.TempDir()
	cfg := &config.Config{
		Paths: config.PathsConfig{CaddyServerMap: filepath.Join(dir, "servers.json")},
	}
	manager := caddy.NewManager(srvURL, cfg.Paths.CaddyServerMap)
	h := NewCaddyHandlerWithManager(cfg, nil, manager, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/caddy/reload", nil)
	rr := httptest.NewRecorder()
	h.Reload(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when Caddy is down, got %d", rr.Code)
	}
}
