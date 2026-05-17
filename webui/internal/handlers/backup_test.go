package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
)

// jsonBody marshals v to JSON and returns a reader for use as an HTTP request body.
func jsonBody(t *testing.T, v interface{}) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("jsonBody: marshal: %v", err)
	}
	return bytes.NewReader(b)
}

// newTestBackupHandler creates a BackupHandler backed by temp directories.
func newTestBackupHandler(t *testing.T) *BackupHandler {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		Paths: config.PathsConfig{
			BackupDir: filepath.Join(dir, "backups"),
		},
		Backup: config.BackupConfig{
			RetentionCount: 5,
		},
	}
	return NewBackupHandler(cfg, nil, nil)
}

// --- APIList ---

func TestBackupHandler_APIList_Empty(t *testing.T) {
	h := newTestBackupHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/backup", nil)
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
	if len(result) != 0 {
		t.Errorf("expected empty list, got %d entries", len(result))
	}
}

// --- Create ---

func TestBackupHandler_Create_WrongMethod(t *testing.T) {
	h := newTestBackupHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/backup/create", nil)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestBackupHandler_Create_Full(t *testing.T) {
	h := newTestBackupHandler(t)
	body := jsonBody(t, map[string]string{"backup_type": "full"})
	req := httptest.NewRequest(http.MethodPost, "/api/backup/create", body)
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
	if resp["filename"] == "" {
		t.Error("expected filename in response")
	}
}

// TestBackupHandler_Create_DefaultsToFull verifies that an invalid/empty body
// triggers a default full backup rather than an error.
func TestBackupHandler_Create_DefaultsToFull(t *testing.T) {
	h := newTestBackupHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/backup/create", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	// The handler falls back to "full" on decode error — should still succeed.
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (fallback to full), got %d (body: %s)", rr.Code, rr.Body.String())
	}
}

// TestBackupHandler_Create_ThenAPIList verifies that a backup appears in the list.
func TestBackupHandler_Create_ThenAPIList(t *testing.T) {
	h := newTestBackupHandler(t)

	// Create a backup.
	createReq := httptest.NewRequest(http.MethodPost, "/api/backup/create",
		jsonBody(t, map[string]string{"backup_type": "full"}))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	h.Create(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("Create failed: %d", createRR.Code)
	}

	// List should now have one entry.
	listReq := httptest.NewRequest(http.MethodGet, "/api/backup", nil)
	listRR := httptest.NewRecorder()
	h.APIList(listRR, listReq)

	var list []config.BackupInfo
	if err := json.NewDecoder(listRR.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 backup, got %d", len(list))
	}
}

// --- Delete ---

func TestBackupHandler_Delete_WrongMethod(t *testing.T) {
	h := newTestBackupHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/backup/delete", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestBackupHandler_Delete_MissingFilename(t *testing.T) {
	h := newTestBackupHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/backup/delete", nil)
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestBackupHandler_Delete_Success(t *testing.T) {
	h := newTestBackupHandler(t)

	// Create a backup first.
	createReq := httptest.NewRequest(http.MethodPost, "/api/backup/create",
		jsonBody(t, map[string]string{"backup_type": "full"}))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	h.Create(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("Create failed: %d", createRR.Code)
	}

	var createResp map[string]interface{}
	if err := json.NewDecoder(createRR.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	filename, _ := createResp["filename"].(string)
	if filename == "" {
		t.Fatal("no filename in create response")
	}

	// Delete it.
	req := httptest.NewRequest(http.MethodPost, "/api/backup/delete?filename="+filename, nil)
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

	// Confirm the list is empty again.
	listReq := httptest.NewRequest(http.MethodGet, "/api/backup", nil)
	listRR := httptest.NewRecorder()
	h.APIList(listRR, listReq)
	var list []config.BackupInfo
	json.NewDecoder(listRR.Body).Decode(&list) //nolint:errcheck
	if len(list) != 0 {
		t.Errorf("expected 0 backups after delete, got %d", len(list))
	}
}

// --- Download ---

func TestBackupHandler_Download_MissingFilename(t *testing.T) {
	h := newTestBackupHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/backup/download", nil)
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestBackupHandler_Download_NotFound(t *testing.T) {
	h := newTestBackupHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/backup/download?filename=ghost.tar.gz", nil)
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestBackupHandler_Download_PathTraversal(t *testing.T) {
	h := newTestBackupHandler(t)
	// Attempt directory traversal — the handler should reject it.
	req := httptest.NewRequest(http.MethodGet, "/api/backup/download?filename=../../etc/passwd", nil)
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	if rr.Code == http.StatusOK {
		t.Error("expected non-200 for path traversal attempt")
	}
}

func TestBackupHandler_Download_Success(t *testing.T) {
	h := newTestBackupHandler(t)

	// Create a backup to download.
	createReq := httptest.NewRequest(http.MethodPost, "/api/backup/create",
		jsonBody(t, map[string]string{"backup_type": "full"}))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	h.Create(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("Create: %d", createRR.Code)
	}
	var createResp map[string]interface{}
	json.NewDecoder(createRR.Body).Decode(&createResp) //nolint:errcheck
	filename, _ := createResp["filename"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/backup/download?filename="+filename, nil)
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "gzip") && !strings.Contains(ct, "octet-stream") {
		t.Errorf("unexpected Content-Type %q for tar.gz download", ct)
	}
}

// --- Upload ---

func TestBackupHandler_Upload_WrongMethod(t *testing.T) {
	h := newTestBackupHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/backup/upload", nil)
	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestBackupHandler_Upload_InvalidExtension(t *testing.T) {
	h := newTestBackupHandler(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("backup", "evil.zip")
	fw.Write([]byte("fake data")) //nolint:errcheck
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/backup/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-.tar.gz file, got %d", rr.Code)
	}
}

func TestBackupHandler_Upload_Success(t *testing.T) {
	h := newTestBackupHandler(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("backup", "test-backup.tar.gz")
	fw.Write([]byte("fake tar.gz content")) //nolint:errcheck
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/backup/upload", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	h.Upload(rr, req)

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
	if resp["filename"] != "test-backup.tar.gz" {
		t.Errorf("expected filename='test-backup.tar.gz', got %v", resp["filename"])
	}
}

// --- Restore ---

func TestBackupHandler_Restore_WrongMethod(t *testing.T) {
	h := newTestBackupHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/backup/restore", nil)
	rr := httptest.NewRecorder()
	h.Restore(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestBackupHandler_Restore_MissingFilename(t *testing.T) {
	h := newTestBackupHandler(t)
	body := jsonBody(t, map[string]string{"filename": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/backup/restore", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Restore(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty filename, got %d", rr.Code)
	}
}

func TestBackupHandler_Restore_NonExistentFile(t *testing.T) {
	h := newTestBackupHandler(t)
	body := jsonBody(t, map[string]string{"filename": "ghost.tar.gz"})
	req := httptest.NewRequest(http.MethodPost, "/api/backup/restore", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Restore(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for non-existent backup, got %d", rr.Code)
	}
}

func TestBackupHandler_Restore_ValidBackup(t *testing.T) {
	h := newTestBackupHandler(t)

	// Create a real backup then restore it.
	createReq := httptest.NewRequest(http.MethodPost, "/api/backup/create",
		jsonBody(t, map[string]string{"backup_type": "full"}))
	createReq.Header.Set("Content-Type", "application/json")
	createRR := httptest.NewRecorder()
	h.Create(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("Create: %d", createRR.Code)
	}

	var createResp map[string]interface{}
	json.NewDecoder(createRR.Body).Decode(&createResp) //nolint:errcheck
	filename, _ := createResp["filename"].(string)

	body := jsonBody(t, map[string]string{"filename": filename})
	req := httptest.NewRequest(http.MethodPost, "/api/backup/restore", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Restore(rr, req)

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
