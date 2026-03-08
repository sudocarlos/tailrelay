package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookieName = "tailrelay_session"
	sessionDuration   = 24 * 3600 // 24 hours in seconds
)

// Middleware provides authentication functionality
type Middleware struct {
	staticToken string // API token for automation

	mu      sync.RWMutex
	session string // Currently active session token
	expiry  time.Time
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(staticToken string) *Middleware {
	return &Middleware{
		staticToken: staticToken,
	}
}

// RequireAuth is middleware that requires authentication
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check 1: Authorization Header (Static API Token)
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			reqToken := strings.TrimPrefix(authHeader, "Bearer ")
			if reqToken == m.staticToken && m.staticToken != "" {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check 2: Valid session cookie
		if m.HasValidSession(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Not authenticated
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

// HasValidSession checks if the request has a valid session cookie
func (m *Middleware) HasValidSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.session == "" || cookie.Value != m.session {
		return false
	}

	if time.Now().After(m.expiry) {
		return false
	}

	return true
}

// CreateSession generates a new session and returns the token
func (m *Middleware) CreateSession() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	m.mu.Lock()
	m.session = token
	m.expiry = time.Now().Add(time.Duration(sessionDuration) * time.Second)
	m.mu.Unlock()

	return token, nil
}

// SetSessionCookie generates a new session and sets the authentication cookie
func (m *Middleware) SetSessionCookie(w http.ResponseWriter, r *http.Request) error {
	token, err := m.CreateSession()
	if err != nil {
		return err
	}

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionDuration,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	if r.TLS != nil || strings.HasPrefix(r.Header.Get("X-Forwarded-Proto"), "https") {
		cookie.Secure = true
	}

	http.SetCookie(w, cookie)
	return nil
}

// ClearSessionCookie clears the authentication session cookie
func (m *Middleware) ClearSessionCookie(w http.ResponseWriter) {
	m.mu.Lock()
	m.session = ""
	m.mu.Unlock()

	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}
