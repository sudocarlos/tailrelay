package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestRequireAuth_ValidBearerToken verifies that a request carrying the correct
// static Bearer token is passed through to the next handler.
func TestRequireAuth_ValidBearerToken(t *testing.T) {
	mw := NewMiddleware("secret-token")
	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	req := httptest.NewRequest(http.MethodGet, "/api/proxies", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rr := httptest.NewRecorder()

	mw.RequireAuth(next).ServeHTTP(rr, req)

	if !reached {
		t.Error("handler not reached with valid Bearer token")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// TestRequireAuth_WrongBearerToken_APIPath verifies that a wrong token on an
// /api/ path returns 401 JSON (not a redirect).
func TestRequireAuth_WrongBearerToken_APIPath(t *testing.T) {
	mw := NewMiddleware("secret-token")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not be reached with wrong token")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/proxies", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rr := httptest.NewRecorder()

	mw.RequireAuth(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json Content-Type, got %q", ct)
	}
}

// TestRequireAuth_NoCredentials_NonAPIPath verifies that an unauthenticated
// request to a non-/api/ path receives a redirect to /login.
func TestRequireAuth_NoCredentials_NonAPIPath(t *testing.T) {
	mw := NewMiddleware("secret-token")
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not be reached without credentials")
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rr := httptest.NewRecorder()

	mw.RequireAuth(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

// TestRequireAuth_ValidSessionCookie verifies that a request with a valid
// session cookie passes through without a Bearer token.
func TestRequireAuth_ValidSessionCookie(t *testing.T) {
	mw := NewMiddleware("secret-token")
	token, err := mw.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rr := httptest.NewRecorder()

	mw.RequireAuth(next).ServeHTTP(rr, req)

	if !reached {
		t.Error("handler not reached with valid session cookie")
	}
}

// TestRequireAuth_ExpiredSession verifies that an expired session cookie is
// rejected and returns 401 for /api/ paths.
func TestRequireAuth_ExpiredSession(t *testing.T) {
	mw := NewMiddleware("secret-token")
	token, err := mw.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Force the session to be expired.
	mw.mu.Lock()
	mw.expiry = time.Now().Add(-1 * time.Second)
	mw.mu.Unlock()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not be reached with expired session")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	rr := httptest.NewRecorder()

	mw.RequireAuth(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired session, got %d", rr.Code)
	}
}

// TestHasValidSession_FreshSessionIsValid checks the obvious happy path.
func TestHasValidSession_FreshSessionIsValid(t *testing.T) {
	mw := NewMiddleware("tok")
	token, err := mw.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})

	if !mw.HasValidSession(req) {
		t.Error("fresh session should be valid")
	}
}

// TestHasValidSession_WrongTokenIsInvalid checks that a mismatched cookie value
// is rejected.
func TestHasValidSession_WrongTokenIsInvalid(t *testing.T) {
	mw := NewMiddleware("tok")
	_, err := mw.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "not-the-right-token"})

	if mw.HasValidSession(req) {
		t.Error("wrong cookie value should not be valid")
	}
}

// TestCreateSession_ReplacesExistingSession verifies that calling CreateSession
// a second time invalidates the first token.
func TestCreateSession_ReplacesExistingSession(t *testing.T) {
	mw := NewMiddleware("")
	first, err := mw.CreateSession()
	if err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}
	second, err := mw.CreateSession()
	if err != nil {
		t.Fatalf("second CreateSession: %v", err)
	}

	if first == second {
		t.Error("two sessions should not produce the same token")
	}

	// The first token is now stale — it should no longer be valid.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: first})
	if mw.HasValidSession(req) {
		t.Error("first session should be invalidated after a new session is created")
	}
}

// TestClearSessionCookie verifies that ClearSessionCookie invalidates the
// current session and sets a Max-Age=-1 cookie.
func TestClearSessionCookie_InvalidatesSession(t *testing.T) {
	mw := NewMiddleware("")
	token, err := mw.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	rr := httptest.NewRecorder()
	mw.ClearSessionCookie(rr)

	// The in-memory session must be cleared.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	if mw.HasValidSession(req) {
		t.Error("session should be invalid after ClearSessionCookie")
	}

	// The response must contain a Set-Cookie that expires the cookie.
	found := false
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookieName && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected an expired Set-Cookie header after ClearSessionCookie")
	}
}

// TestSetSessionCookie_SetsCorrectAttributes verifies that the cookie written
// by SetSessionCookie is HttpOnly, has SameSite=Strict, and the Path is "/".
func TestSetSessionCookie_SetsCorrectAttributes(t *testing.T) {
	mw := NewMiddleware("")

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rr := httptest.NewRecorder()

	if err := mw.SetSessionCookie(rr, req); err != nil {
		t.Fatalf("SetSessionCookie: %v", err)
	}

	cookies := rr.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected at least one Set-Cookie header")
	}
	c := cookies[0]
	if c.Name != sessionCookieName {
		t.Errorf("unexpected cookie name %q", c.Name)
	}
	if !c.HttpOnly {
		t.Error("cookie should be HttpOnly")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSite=Strict, got %v", c.SameSite)
	}
	if c.Path != "/" {
		t.Errorf("expected Path=/, got %q", c.Path)
	}
}
