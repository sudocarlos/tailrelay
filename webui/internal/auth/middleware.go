package auth

import (
	"log"
	"net"
	"net/http"
	"strings"
)

const (
	sessionCookieName = "tailrelay_session"
	sessionDuration   = 24 * 3600 // 24 hours in seconds
)

// Middleware provides authentication functionality
type Middleware struct {
	token               string
	enableTailscaleAuth bool
	enableTokenAuth     bool
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(token string, enableTailscaleAuth, enableTokenAuth bool) *Middleware {
	return &Middleware{
		token:               token,
		enableTailscaleAuth: enableTailscaleAuth,
		enableTokenAuth:     enableTokenAuth,
	}
}

// RequireAuth is middleware that requires authentication
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check 1: Valid session cookie
		if m.enableTokenAuth && m.hasValidSession(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Check 2: Request from Tailscale IP
		if m.enableTailscaleAuth && m.isTailscaleIP(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Not authenticated - redirect to login
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

// hasValidSession checks if the request has a valid session cookie
func (m *Middleware) hasValidSession(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}

	// Simple token validation
	return cookie.Value == m.token
}

// isTailscaleIP checks if the request comes from a Tailscale IP
func (m *Middleware) isTailscaleIP(r *http.Request) bool {
	// Get remote IP address
	remoteAddr := r.RemoteAddr
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// If no port, use the whole address
		host = remoteAddr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		log.Printf("Failed to parse IP: %s", host)
		return false
	}

	// Check for Tailscale IPv4 range (100.64.0.0/10)
	_, tailscaleV4, _ := net.ParseCIDR("100.64.0.0/10")
	if tailscaleV4.Contains(ip) {
		log.Printf("Authenticated via Tailscale IPv4: %s", ip.String())
		return true
	}

	// Check for Tailscale IPv6 range (fd7a:115c:a1e0::/48)
	_, tailscaleV6, _ := net.ParseCIDR("fd7a:115c:a1e0::/48")
	if tailscaleV6.Contains(ip) {
		log.Printf("Authenticated via Tailscale IPv6: %s", ip.String())
		return true
	}

	return false
}

// SetSessionCookie sets the authentication session cookie
func (m *Middleware) SetSessionCookie(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    m.token,
		Path:     "/",
		MaxAge:   sessionDuration,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	// Set Secure flag if using HTTPS
	if r.TLS != nil || strings.HasPrefix(r.Header.Get("X-Forwarded-Proto"), "https") {
		cookie.Secure = true
	}

	http.SetCookie(w, cookie)
}

// ClearSessionCookie clears the authentication session cookie
func (m *Middleware) ClearSessionCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
}

// ValidateToken checks if the provided token matches the configured token
func (m *Middleware) ValidateToken(token string) bool {
	return m.enableTokenAuth && token == m.token
}
