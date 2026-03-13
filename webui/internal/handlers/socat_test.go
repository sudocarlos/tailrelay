package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/socat"
)

// newTestSocatHandler creates a SocatHandler backed by a temp directory.
// The socat binary is set to a non-existent path so that any accidental
// process-start calls fail immediately rather than spawning real processes.
func newTestSocatHandler(t *testing.T) (*SocatHandler, string) {
	t.Helper()
	dir := t.TempDir()
	relaysFile := filepath.Join(dir, "relays.json")
	cfg := &config.Config{
		Paths: config.PathsConfig{
			SocatRelayConfig: relaysFile,
		},
	}
	manager := socat.NewManager("/nonexistent/socat-binary", relaysFile)
	h := &SocatHandler{
		cfg:       cfg,
		templates: nil,
		manager:   manager,
	}
	return h, relaysFile
}

// seedRelay writes a relay directly to the relays file and returns it.
func seedRelay(t *testing.T, relaysFile string, relay config.SocatRelay) config.SocatRelay {
	t.Helper()
	if err := socat.AddRelay(relaysFile, relay); err != nil {
		t.Fatalf("seedRelay: %v", err)
	}
	return relay
}

// --- APIList ---

func TestSocatHandler_APIList_Empty(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/socat", nil)
	rr := httptest.NewRecorder()
	h.APIList(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Errorf("expected application/json, got %q", rr.Header().Get("Content-Type"))
	}
	var result []interface{}
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("response is not a JSON array: %v", err)
	}
}

func TestSocatHandler_APIList_WithRelays(t *testing.T) {
	h, relaysFile := newTestSocatHandler(t)
	seedRelay(t, relaysFile, config.SocatRelay{
		ID:         "r1",
		ListenPort: 5001,
		TargetHost: "127.0.0.1",
		TargetPort: 6001,
		Enabled:    false,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/socat", nil)
	rr := httptest.NewRecorder()
	h.APIList(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var result []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 relay, got %d", len(result))
	}
}

// --- APIGet ---

func TestSocatHandler_APIGet_MissingID(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/socat/get", nil)
	rr := httptest.NewRecorder()
	h.APIGet(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSocatHandler_APIGet_NotFound(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/socat/get?id=nope", nil)
	rr := httptest.NewRecorder()
	h.APIGet(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestSocatHandler_APIGet_Found(t *testing.T) {
	h, relaysFile := newTestSocatHandler(t)
	seedRelay(t, relaysFile, config.SocatRelay{
		ID:         "get-r1",
		ListenPort: 5010,
		TargetHost: "127.0.0.1",
		TargetPort: 6010,
		Enabled:    false,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/socat/get?id=get-r1", nil)
	rr := httptest.NewRecorder()
	h.APIGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	var relay config.SocatRelay
	if err := json.NewDecoder(rr.Body).Decode(&relay); err != nil {
		t.Fatalf("decode relay: %v", err)
	}
	if relay.ID != "get-r1" {
		t.Errorf("expected id 'get-r1', got %q", relay.ID)
	}
}

// --- Create ---

func TestSocatHandler_Create_WrongMethod(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/socat/create", nil)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestSocatHandler_Create_InvalidBody(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/socat/create", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestSocatHandler_Create_DisabledRelay verifies that creating a disabled relay
// (Enabled: false) succeeds without attempting to spawn a socat process.
func TestSocatHandler_Create_DisabledRelay(t *testing.T) {
	h, relaysFile := newTestSocatHandler(t)

	body := jsonBody(t, config.SocatRelay{
		ID:         "create-r1",
		ListenPort: 5020,
		TargetHost: "127.0.0.1",
		TargetPort: 6020,
		Enabled:    false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/socat/create", body)
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

	// Verify persisted.
	relay, err := socat.GetRelay(relaysFile, "create-r1")
	if err != nil {
		t.Fatalf("relay not persisted: %v", err)
	}
	if relay.ListenPort != 5020 {
		t.Errorf("expected ListenPort 5020, got %d", relay.ListenPort)
	}
}

// --- Update ---

func TestSocatHandler_Update_WrongMethod(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/socat/update", nil)
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestSocatHandler_Update_MissingID(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	body := jsonBody(t, config.SocatRelay{ListenPort: 5000, TargetHost: "127.0.0.1", TargetPort: 6000})
	req := httptest.NewRequest(http.MethodPost, "/api/socat/update", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestSocatHandler_Update_NotFound(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	body := jsonBody(t, config.SocatRelay{
		ID:         "ghost",
		ListenPort: 5000,
		TargetHost: "127.0.0.1",
		TargetPort: 6000,
		Enabled:    false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/socat/update", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent relay, got %d", rr.Code)
	}
}

// TestSocatHandler_Update_DisabledRelay verifies that updating a disabled relay
// (PID 0) succeeds without stopping/starting processes.
func TestSocatHandler_Update_DisabledRelay(t *testing.T) {
	h, relaysFile := newTestSocatHandler(t)
	seedRelay(t, relaysFile, config.SocatRelay{
		ID:         "upd-r1",
		ListenPort: 5030,
		TargetHost: "127.0.0.1",
		TargetPort: 6030,
		Enabled:    false,
	})

	body := jsonBody(t, config.SocatRelay{
		ID:         "upd-r1",
		ListenPort: 5031,
		TargetHost: "127.0.0.1",
		TargetPort: 6031,
		Enabled:    false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/socat/update", body)
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

	// Verify the port was actually updated.
	relay, err := socat.GetRelay(relaysFile, "upd-r1")
	if err != nil {
		t.Fatalf("get relay: %v", err)
	}
	if relay.ListenPort != 5031 {
		t.Errorf("expected ListenPort 5031, got %d", relay.ListenPort)
	}
}

// --- Delete ---

func TestSocatHandler_Delete_WrongMethod(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/socat/delete", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestSocatHandler_Delete_MissingID(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/socat/delete", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing id, got %d", rr.Code)
	}
}

func TestSocatHandler_Delete_NotFound(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/socat/delete?id=nonexistent", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent relay, got %d", rr.Code)
	}
}

// TestSocatHandler_Delete_DisabledRelay verifies deleting a relay with PID 0
// (no running process) succeeds without trying to stop anything.
func TestSocatHandler_Delete_DisabledRelay(t *testing.T) {
	h, relaysFile := newTestSocatHandler(t)
	seedRelay(t, relaysFile, config.SocatRelay{
		ID:         "del-r1",
		ListenPort: 5040,
		TargetHost: "127.0.0.1",
		TargetPort: 6040,
		Enabled:    false,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/socat/delete?id=del-r1", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// Verify it's gone.
	_, err := socat.GetRelay(relaysFile, "del-r1")
	if err == nil {
		t.Error("relay should have been deleted")
	}
}

// --- Toggle ---

func TestSocatHandler_Toggle_WrongMethod(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/socat/toggle", nil)
	rr := httptest.NewRecorder()
	h.Toggle(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestSocatHandler_Toggle_MissingID(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	body := jsonBody(t, map[string]interface{}{"id": "", "enabled": false})
	req := httptest.NewRequest(http.MethodPost, "/api/socat/toggle", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Toggle(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing id, got %d", rr.Code)
	}
}

func TestSocatHandler_Toggle_NotFound(t *testing.T) {
	h, _ := newTestSocatHandler(t)
	body := jsonBody(t, map[string]interface{}{"id": "ghost", "enabled": false})
	req := httptest.NewRequest(http.MethodPost, "/api/socat/toggle", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Toggle(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent relay, got %d", rr.Code)
	}
}

// TestSocatHandler_Toggle_DisableRunning verifies that disabling a relay with
// PID 0 (already stopped) succeeds without process interaction.
func TestSocatHandler_Toggle_DisableAlreadyStopped(t *testing.T) {
	h, relaysFile := newTestSocatHandler(t)
	seedRelay(t, relaysFile, config.SocatRelay{
		ID:         "tog-r1",
		ListenPort: 5050,
		TargetHost: "127.0.0.1",
		TargetPort: 6050,
		Enabled:    true,
		// PID 0 — not running
	})

	body := jsonBody(t, map[string]interface{}{"id": "tog-r1", "enabled": false})
	req := httptest.NewRequest(http.MethodPost, "/api/socat/toggle", body)
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

	// Verify disabled state persisted.
	relay, err := socat.GetRelay(relaysFile, "tog-r1")
	if err != nil {
		t.Fatalf("get relay: %v", err)
	}
	if relay.Enabled {
		t.Error("expected relay to be disabled after toggle")
	}
}
