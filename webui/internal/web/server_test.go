package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestStaticFileHandler(t *testing.T) {
	// Create a mock filesystem with test files
	mockFS := fstest.MapFS{
		"test.svg": &fstest.MapFile{
			Data: []byte("<svg></svg>"),
		},
		"test.js": &fstest.MapFile{
			Data: []byte("console.log('test');"),
		},
		"test.css": &fstest.MapFile{
			Data: []byte("body { margin: 0; }"),
		},
		"test.html": &fstest.MapFile{
			Data: []byte("<html></html>"),
		},
	}

	// Create a server with mock filesystem
	server := &Server{
		staticFS: mockFS,
	}

	// Create file server handler
	fileServer := http.FileServer(http.FS(server.staticFS))
	handler := server.staticFileHandler(fileServer)

	tests := []struct {
		name         string
		path         string
		expectedType string
	}{
		{
			name:         "SVG file",
			path:         "/test.svg",
			expectedType: "image/svg+xml",
		},
		{
			name:         "JavaScript file",
			path:         "/test.js",
			expectedType: "application/javascript",
		},
		{
			name:         "CSS file",
			path:         "/test.css",
			expectedType: "text/css",
		},
		{
			name:         "HTML file (no override)",
			path:         "/test.html",
			expectedType: "", // Should not be overridden
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			contentType := w.Header().Get("Content-Type")

			if tt.expectedType != "" {
				if contentType != tt.expectedType {
					t.Errorf("Expected Content-Type %q, got %q", tt.expectedType, contentType)
				}
			}

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

func TestStaticFileHandler_BootstrapIcons(t *testing.T) {
	// Test specifically for Bootstrap Icons path
	mockFS := fstest.MapFS{
		"vendor/bootstrap-icons/bootstrap-icons.svg": &fstest.MapFile{
			Data: []byte("<svg><symbol id=\"test\"></symbol></svg>"),
		},
	}

	server := &Server{
		staticFS: mockFS,
	}

	fileServer := http.FileServer(http.FS(server.staticFS))
	handler := server.staticFileHandler(fileServer)

	req := httptest.NewRequest(http.MethodGet, "/vendor/bootstrap-icons/bootstrap-icons.svg", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "image/svg+xml" {
		t.Errorf("Bootstrap Icons SVG should have Content-Type image/svg+xml, got %q", contentType)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestSPARedirect(t *testing.T) {
	// handleSPARedirect needs no collaborators, so a zero Server is enough.
	server := &Server{}

	tests := []struct {
		name string
		path string
	}{
		{"Legacy logs page", "/logs"},
		{"Legacy tailscale page", "/tailscale"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			server.handleSPARedirect(w, req)

			if w.Code != http.StatusSeeOther {
				t.Errorf("Expected status 303, got %d", w.Code)
			}
			if loc := w.Header().Get("Location"); loc != "/" {
				t.Errorf("Expected redirect to /, got %q", loc)
			}
		})
	}
}

func TestSPARedirect_RejectsNonGet(t *testing.T) {
	server := &Server{}

	req := httptest.NewRequest(http.MethodPost, "/logs", nil)
	w := httptest.NewRecorder()

	server.handleSPARedirect(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}
