package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/sudocarlos/tailrelay/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	authMW        *auth.Middleware
	adminHashFile string
}

func NewAuthHandler(authMW *auth.Middleware, adminHashFile string) *AuthHandler {
	return &AuthHandler{
		authMW:        authMW,
		adminHashFile: adminHashFile,
	}
}

func (h *AuthHandler) needsSetup() bool {
	if _, err := os.Stat(h.adminHashFile); os.IsNotExist(err) {
		return true
	}
	return false
}

func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	needsSetup := h.needsSetup()
	authenticated := h.authMW.HasValidSession(r)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"needsSetup":    needsSetup,
		"authenticated": authenticated,
	})
}

func (h *AuthHandler) Setup(w http.ResponseWriter, r *http.Request) {
	if !h.needsSetup() {
		http.Error(w, "Setup already completed", http.StatusForbidden)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Password == "" {
		http.Error(w, "Password cannot be empty", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(h.adminHashFile, hash, 0600); err != nil {
		http.Error(w, "Failed to save password", http.StatusInternalServerError)
		return
	}

	if err := h.authMW.SetSessionCookie(w, r); err != nil {
		http.Error(w, "Failed to set session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.needsSetup() {
		http.Error(w, "System needs setup first", http.StatusForbidden)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	hash, err := os.ReadFile(h.adminHashFile)
	if err != nil {
		http.Error(w, "Failed to read auth config", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(req.Password)); err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	if err := h.authMW.SetSessionCookie(w, r); err != nil {
		http.Error(w, "Failed to set session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.authMW.ClearSessionCookie(w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if h.needsSetup() {
		http.Error(w, "System needs setup first", http.StatusForbidden)
		return
	}

	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "Current and new passwords are required", http.StatusBadRequest)
		return
	}

	hash, err := os.ReadFile(h.adminHashFile)
	if err != nil {
		http.Error(w, "Failed to read auth config", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword(hash, []byte(req.CurrentPassword)); err != nil {
		http.Error(w, "Current password is incorrect", http.StatusUnauthorized)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash new password", http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(h.adminHashFile, newHash, 0600); err != nil {
		http.Error(w, "Failed to save new password", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
