package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

// newTestAuthHandler creates an AuthHandler backed by a temp directory.
// The returned cleanup function removes the temp dir; call it in defer.
func newTestAuthHandler(t *testing.T) (*AuthHandler, string) {
	t.Helper()
	dir := t.TempDir()
	hashFile := filepath.Join(dir, "admin.hash")
	mw := auth.NewMiddleware("static-token")
	return NewAuthHandler(mw, hashFile), hashFile
}

// writePassword hashes pw and writes it to hashFile, simulating a completed setup.
func writePassword(t *testing.T, hashFile, pw string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	if err := os.WriteFile(hashFile, hash, 0600); err != nil {
		t.Fatalf("write hash file: %v", err)
	}
}

// postJSON sends a POST with a JSON body and returns the ResponseRecorder.
func postJSON(t *testing.T, handler http.HandlerFunc, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler(rr, req)
	return rr
}

// --- Status ---

func TestAuthHandler_Status_BeforeSetup(t *testing.T) {
	h, _ := newTestAuthHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["needsSetup"] != true {
		t.Errorf("needsSetup should be true before setup, got %v", resp["needsSetup"])
	}
	if resp["authenticated"] != false {
		t.Errorf("authenticated should be false before login, got %v", resp["authenticated"])
	}
}

func TestAuthHandler_Status_AfterSetup(t *testing.T) {
	h, hashFile := newTestAuthHandler(t)
	writePassword(t, hashFile, "pass123")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rr := httptest.NewRecorder()
	h.Status(rr, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["needsSetup"] != false {
		t.Errorf("needsSetup should be false after setup, got %v", resp["needsSetup"])
	}
}

// --- Setup ---

func TestAuthHandler_Setup_Success(t *testing.T) {
	h, hashFile := newTestAuthHandler(t)
	rr := postJSON(t, h.Setup, "/api/auth/setup", map[string]string{"password": "MyPassword1"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(hashFile); err != nil {
		t.Error("hash file should exist after setup")
	}
	// A session cookie should have been set.
	hasCookie := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "tailrelay_session" {
			hasCookie = true
		}
	}
	if !hasCookie {
		t.Error("expected session cookie to be set after setup")
	}
}

func TestAuthHandler_Setup_EmptyPasswordRejected(t *testing.T) {
	h, _ := newTestAuthHandler(t)
	rr := postJSON(t, h.Setup, "/api/auth/setup", map[string]string{"password": ""})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty password, got %d", rr.Code)
	}
}

func TestAuthHandler_Setup_AlreadySetupRejected(t *testing.T) {
	h, hashFile := newTestAuthHandler(t)
	writePassword(t, hashFile, "existing")

	rr := postJSON(t, h.Setup, "/api/auth/setup", map[string]string{"password": "newpass"})

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 when setup already complete, got %d", rr.Code)
	}
}

// --- Login ---

func TestAuthHandler_Login_Success(t *testing.T) {
	h, hashFile := newTestAuthHandler(t)
	writePassword(t, hashFile, "correct-pass")

	rr := postJSON(t, h.Login, "/api/auth/login", map[string]string{"password": "correct-pass"})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	hasCookie := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "tailrelay_session" {
			hasCookie = true
		}
	}
	if !hasCookie {
		t.Error("expected session cookie after successful login")
	}
}

func TestAuthHandler_Login_WrongPassword(t *testing.T) {
	h, hashFile := newTestAuthHandler(t)
	writePassword(t, hashFile, "correct-pass")

	rr := postJSON(t, h.Login, "/api/auth/login", map[string]string{"password": "wrong-pass"})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", rr.Code)
	}
}

func TestAuthHandler_Login_BeforeSetup(t *testing.T) {
	h, _ := newTestAuthHandler(t)
	rr := postJSON(t, h.Login, "/api/auth/login", map[string]string{"password": "anything"})

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 before setup, got %d", rr.Code)
	}
}

// --- Logout ---

func TestAuthHandler_Logout_ClearsSession(t *testing.T) {
	h, hashFile := newTestAuthHandler(t)
	writePassword(t, hashFile, "pass")

	// Login first to obtain a session.
	loginRR := postJSON(t, h.Login, "/api/auth/login", map[string]string{"password": "pass"})
	if loginRR.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginRR.Code)
	}

	// Logout.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 from logout, got %d", rr.Code)
	}

	// The session cookie must be cleared (MaxAge < 0).
	cleared := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == "tailrelay_session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("expected session cookie to be expired/cleared after logout")
	}
}

// --- ChangePassword ---

func TestAuthHandler_ChangePassword_Success(t *testing.T) {
	h, hashFile := newTestAuthHandler(t)
	writePassword(t, hashFile, "old-pass")

	rr := postJSON(t, h.ChangePassword, "/api/auth/change-password", map[string]string{
		"currentPassword": "old-pass",
		"newPassword":     "new-pass",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", rr.Code, rr.Body.String())
	}

	// Verify the new password is accepted.
	hash, _ := os.ReadFile(hashFile)
	if bcrypt.CompareHashAndPassword(hash, []byte("new-pass")) != nil {
		t.Error("new password should be accepted after change")
	}
}

func TestAuthHandler_ChangePassword_WrongCurrentPassword(t *testing.T) {
	h, hashFile := newTestAuthHandler(t)
	writePassword(t, hashFile, "real-pass")

	rr := postJSON(t, h.ChangePassword, "/api/auth/change-password", map[string]string{
		"currentPassword": "wrong",
		"newPassword":     "new",
	})

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong current password, got %d", rr.Code)
	}
}

func TestAuthHandler_ChangePassword_BeforeSetup(t *testing.T) {
	h, _ := newTestAuthHandler(t)

	rr := postJSON(t, h.ChangePassword, "/api/auth/change-password", map[string]string{
		"currentPassword": "old",
		"newPassword":     "new",
	})

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 before setup, got %d", rr.Code)
	}
}
