package web

import (
	"context"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sudocarlos/tailrelay/internal/auth"
	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/handlers"
)

// Server represents the HTTP server
type Server struct {
	cfg        *config.Config
	authMW     *auth.Middleware
	templates  *template.Template
	authH      *handlers.AuthHandler
	dashboardH *handlers.DashboardHandler
	tailscaleH *handlers.TailscaleHandler
	serveH     *handlers.ServeHandler
	backupH    *handlers.BackupHandler
	targetsH   *handlers.TargetsHandler
	logsH      *handlers.Handler
	infoH      *handlers.InfoHandler
	distFS     fs.FS
	staticFS   fs.FS
	templateFS fs.FS
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, authToken, version, commit string, distFS, staticFS, templateFS fs.FS) (*Server, error) {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Create authentication middleware
	authMW := auth.NewMiddleware(authToken)

	// Parse templates (gracefully handle missing templates for new SPA mode)
	var tmpl *template.Template
	if templateFS != nil {
		var err error
		tmpl, err = loadTemplates(templateFS)
		if err != nil {
			log.Printf("Warning: failed to load templates (SPA mode): %v", err)
		}
	}
	if tmpl == nil {
		tmpl = template.New("")
	}

	// Create handlers
	authH := handlers.NewAuthHandler(authMW, cfg.Auth.AdminHashFile)
	dashboardH := handlers.NewDashboardHandler(cfg, tmpl)

	serveH := handlers.NewServeHandler(cfg, tmpl)
	tailscaleH := handlers.NewTailscaleHandler(cfg, tmpl, authMW, serveH.Manager())
	backupH := handlers.NewBackupHandler(cfg, tmpl, serveH.Manager())
	targetsH := handlers.NewTargetsHandler(cfg)
	logsH := handlers.NewHandler(tmpl)
	infoH := handlers.NewInfoHandler(version, commit)

	return &Server{
		cfg:        cfg,
		authMW:     authMW,
		authH:      authH,
		templates:  tmpl,
		dashboardH: dashboardH,
		tailscaleH: tailscaleH,
		serveH:     serveH,
		backupH:    backupH,
		targetsH:   targetsH,
		logsH:      logsH,
		infoH:      infoH,
		distFS:     distFS,
		staticFS:   staticFS,
		templateFS: templateFS,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Initialize tailscale serve relay state asynchronously,
	// retrying until the tailscaled netMap is fully loaded and ready.
	go func() {
		log.Printf("Waiting for tailscale to connect before reconciling relays...")
		for i := 0; i < 15; i++ {
			if s.serveH.IsTailscaleReady() {
				if err := s.serveH.InitializeAutostart(); err == nil {
					log.Printf("Successfully reconciled tailscale serve relays")
					return
				} else {
					log.Printf("Warning: failed to reconcile tailscale serve relays on attempt %d: %v", i+1, err)
				}
			}

			if i < 14 {
				time.Sleep(2 * time.Second)
			} else {
				log.Printf("Warning: tailscale failed to connect or reconcile relays after multiple attempts")
			}
		}
	}()

	mux := s.setupRoutes()

	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	log.Printf("Starting Web UI server on %s", addr)

	// Create HTTP server for graceful shutdown support
	httpServer := &http.Server{
		Addr:    addr,
		Handler: securityHeaders(mux),
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- httpServer.ListenAndServe()
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			s.cancel()
			return err
		}
	case sig := <-sigChan:
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)

		// Cancel context to stop monitor goroutines
		s.cancel()

		// Graceful shutdown of HTTP server (30 second timeout)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
			return err
		}

		log.Printf("Server stopped gracefully")
	}

	return nil
}

// securityHeaders wraps a handler and adds defensive HTTP security headers.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Public static assets — served without auth so browsers can fetch
	// favicons, PWA icons, and the web manifest before/without a session.
	for _, asset := range []struct{ path, mime string }{
		{"/favicon.ico", "image/x-icon"},
		{"/favicon.png", "image/png"},
		{"/apple-touch-icon.png", "image/png"},
		{"/icon-192.png", "image/png"},
		{"/icon-512.png", "image/png"},
		{"/manifest.webmanifest", "application/manifest+json"},
	} {
		asset := asset
		mux.HandleFunc(asset.path, func(w http.ResponseWriter, r *http.Request) {
			s.serveDistFile(w, r, asset.path[1:], asset.mime)
		})
	}

	// Public routes (no authentication required)
	mux.Handle("/api/info", http.HandlerFunc(s.infoH.Info))
	mux.Handle("/api/auth/status", http.HandlerFunc(s.authH.Status))
	mux.Handle("/api/auth/setup", http.HandlerFunc(s.authH.Setup))
	mux.Handle("/api/auth/login", http.HandlerFunc(s.authH.Login))

	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)

	// Vite dist assets (hashed filenames, long-cache)
	if s.distFS != nil {
		distFileServer := http.FileServer(http.FS(s.distFS))
		mux.Handle("/assets/", s.cacheableFileHandler(distFileServer))
	}

	// Legacy static files (backward compat for old templates still in use)
	if s.staticFS != nil {
		fileServer := http.FileServer(http.FS(s.staticFS))
		mux.Handle("/static/", http.StripPrefix("/static/", s.staticFileHandler(fileServer)))
	}

	// Legacy endpoint shims — removed endpoints return 410 Gone with migration hint.
	// These are registered before the protected catch-all so they respond without auth.
	legacyGone := func(newBase string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusGone)
			_, _ = w.Write([]byte("This endpoint has been removed. Use " + newBase + " instead.\n"))
		}
	}
	mux.HandleFunc("/api/caddy/", legacyGone("/api/serve/https/"))
	mux.HandleFunc("/api/socat/", legacyGone("/api/serve/tcp/"))

	// Protected routes (authentication required)
	mux.Handle("/", s.authMW.RequireAuth(http.HandlerFunc(s.handleSPAFallback)))
	mux.Handle("/api/auth/logout", s.authMW.RequireAuth(http.HandlerFunc(s.authH.Logout)))
	mux.Handle("/api/auth/change-password", s.authMW.RequireAuth(http.HandlerFunc(s.authH.ChangePassword)))
	mux.Handle("/api/status", s.authMW.RequireAuth(http.HandlerFunc(s.dashboardH.APIStatus)))
	mux.Handle("/api/targets", s.authMW.RequireAuth(http.HandlerFunc(s.targetsH.APIList)))

	// Tailscale routes
	mux.Handle("/tailscale", s.authMW.RequireAuth(http.HandlerFunc(s.handleSPARedirect)))
	mux.Handle("/api/tailscale/login", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.Login)))
	mux.Handle("/api/tailscale/login-with-key", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.LoginWithKey)))
	mux.Handle("/api/tailscale/logout", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.Logout)))
	mux.Handle("/api/tailscale/connect", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.Connect)))
	mux.Handle("/api/tailscale/disconnect", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.Disconnect)))
	mux.Handle("/api/tailscale/hostname", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.ChangeHostname)))
	mux.Handle("/api/tailscale/status", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.APIStatus)))
	mux.Handle("/api/tailscale/peers", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.APIPeers)))
	mux.Handle("/api/tailscale/poll", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.PollStatus)))

	// HTTPS relay routes (/api/serve/https/*)
	mux.Handle("/api/serve/https/list", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.APIListHTTPS)))
	mux.Handle("/api/serve/https/get", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.APIGetHTTPS)))
	mux.Handle("/api/serve/https/create", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.CreateHTTPS)))
	mux.Handle("/api/serve/https/update", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.UpdateHTTPS)))
	mux.Handle("/api/serve/https/delete", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.DeleteHTTPS)))
	mux.Handle("/api/serve/https/toggle", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.ToggleHTTPS)))

	// TCP relay routes (/api/serve/tcp/*)
	mux.Handle("/api/serve/tcp/list", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.APIListTCP)))
	mux.Handle("/api/serve/tcp/get", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.APIGetTCP)))
	mux.Handle("/api/serve/tcp/create", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.CreateTCP)))
	mux.Handle("/api/serve/tcp/update", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.UpdateTCP)))
	mux.Handle("/api/serve/tcp/delete", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.DeleteTCP)))
	mux.Handle("/api/serve/tcp/toggle", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.ToggleTCP)))
	mux.Handle("/api/serve/reload", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.ReloadServe)))

	// Funnel relay routes (/api/serve/funnel/*)
	mux.Handle("/api/serve/funnel/list", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.APIListFunnel)))
	mux.Handle("/api/serve/funnel/get", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.APIGetFunnel)))
	mux.Handle("/api/serve/funnel/create", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.CreateFunnel)))
	mux.Handle("/api/serve/funnel/update", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.UpdateFunnel)))
	mux.Handle("/api/serve/funnel/delete", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.DeleteFunnel)))
	mux.Handle("/api/serve/funnel/toggle", s.authMW.RequireAuth(http.HandlerFunc(s.serveH.ToggleFunnel)))

	// Backup routes
	mux.Handle("/backup", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.List)))
	mux.Handle("/api/backup/create", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.Create)))
	mux.Handle("/api/backup/restore", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.Restore)))
	mux.Handle("/api/backup/delete", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.Delete)))
	mux.Handle("/api/backup/rename", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.Rename)))
	mux.Handle("/api/backup/download", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.Download)))
	mux.Handle("/api/backup/upload", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.Upload)))
	mux.Handle("/api/backup/list", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.APIList)))

	// Logs routes
	mux.Handle("/logs", s.authMW.RequireAuth(http.HandlerFunc(s.logsH.LogsPageHandler)))
	mux.Handle("/api/logs", s.authMW.RequireAuth(http.HandlerFunc(s.logsH.LogsAPIHandler)))
	mux.Handle("/api/logs/stream", s.authMW.RequireAuth(http.HandlerFunc(s.logsH.LogsStreamHandler)))
	mux.Handle("/api/logs/level", s.authMW.RequireAuth(http.HandlerFunc(s.logsH.LogsLevelHandler)))

	return mux
}

// handleSPAFallback serves the SPA shell for non-API GET requests.
// It reads index.html from the Vite dist FS and serves it directly.
func (s *Server) handleSPAFallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	s.serveDistIndex(w, r)
}

// handleSPARedirect sends legacy pages to the SPA.
func (s *Server) handleSPARedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogin serves the SPA shell at /login (the SPA handles auth state).
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.serveDistIndex(w, r)
}

// handleLogout handles the logout action
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.authMW.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// loadTemplates loads and parses all HTML templates
func loadTemplates(templateFS fs.FS) (*template.Template, error) {
	// Create template with helper functions
	tmpl := template.New("").Funcs(template.FuncMap{
		"formatSize": formatSize,
	})

	// Parse all templates from embedded filesystem
	err := fs.WalkDir(templateFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && (len(path) > 5 && path[len(path)-5:] == ".html") {
			content, err := fs.ReadFile(templateFS, path)
			if err != nil {
				return err
			}

			_, err = tmpl.New(d.Name()).Parse(string(content))
			if err != nil {
				return fmt.Errorf("failed to parse template %s: %w", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

// staticFileHandler wraps the file server to set correct MIME types for static assets
func (s *Server) staticFileHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set correct MIME type for SVG files
		if len(r.URL.Path) > 4 && r.URL.Path[len(r.URL.Path)-4:] == ".svg" {
			w.Header().Set("Content-Type", "image/svg+xml")
		}
		// Set correct MIME type for JavaScript files
		if len(r.URL.Path) > 3 && r.URL.Path[len(r.URL.Path)-3:] == ".js" {
			w.Header().Set("Content-Type", "application/javascript")
		}
		// Set correct MIME type for CSS files
		if len(r.URL.Path) > 4 && r.URL.Path[len(r.URL.Path)-4:] == ".css" {
			w.Header().Set("Content-Type", "text/css")
		}
		next.ServeHTTP(w, r)
	})
}

// formatSize formats bytes into human-readable size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// serveDistFile reads a named file from the Vite dist FS and writes it
// with the given Content-Type. Used for public static assets that must be
// reachable without authentication (favicons, PWA icons, manifest).
func (s *Server) serveDistFile(w http.ResponseWriter, r *http.Request, name, mime string) {
	if s.distFS == nil {
		http.NotFound(w, r)
		return
	}
	content, err := fs.ReadFile(s.distFS, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(content)
}

// serveDistIndex reads index.html from the Vite dist FS and writes it
// as the response. This is used for SPA fallback and the login page.
func (s *Server) serveDistIndex(w http.ResponseWriter, r *http.Request) {
	if s.distFS == nil {
		http.Error(w, "SPA assets not available", http.StatusInternalServerError)
		return
	}

	content, err := fs.ReadFile(s.distFS, "index.html")
	if err != nil {
		log.Printf("Error reading dist/index.html: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(content)
}

// cacheableFileHandler sets aggressive cache headers for Vite's hashed assets.
func (s *Server) cacheableFileHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vite assets have content hashes in filenames — cache for 1 year
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

		// Set correct MIME types
		path := r.URL.Path
		if strings.HasSuffix(path, ".js") {
			w.Header().Set("Content-Type", "application/javascript")
		} else if strings.HasSuffix(path, ".css") {
			w.Header().Set("Content-Type", "text/css")
		} else if strings.HasSuffix(path, ".svg") {
			w.Header().Set("Content-Type", "image/svg+xml")
		}

		next.ServeHTTP(w, r)
	})
}
