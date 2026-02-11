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

	"github.com/sudocarlos/tailrelay-webui/internal/auth"
	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/handlers"
)

// Server represents the HTTP server
type Server struct {
	cfg        *config.Config
	authMW     *auth.Middleware
	templates  *template.Template
	dashboardH *handlers.DashboardHandler
	tailscaleH *handlers.TailscaleHandler
	caddyH     *handlers.CaddyHandler
	socatH     *handlers.SocatHandler
	backupH    *handlers.BackupHandler
	logsH      *handlers.Handler
	staticFS   fs.FS
	templateFS fs.FS
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewServer creates a new HTTP server
func NewServer(cfg *config.Config, authToken string, staticFS, templateFS fs.FS) (*Server, error) {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Create authentication middleware
	authMW := auth.NewMiddleware(
		authToken,
		cfg.Auth.EnableTailscaleAuth,
		cfg.Auth.EnableTokenAuth,
	)

	// Parse templates
	tmpl, err := loadTemplates(templateFS)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	// Create handlers
	dashboardH := handlers.NewDashboardHandler(cfg, tmpl)
	tailscaleH := handlers.NewTailscaleHandler(cfg, tmpl, authMW)
	caddyH := handlers.NewCaddyHandler(cfg, tmpl)
	socatH := handlers.NewSocatHandler(cfg, tmpl)
	backupH := handlers.NewBackupHandler(cfg, tmpl)
	logsH := handlers.NewHandler(tmpl)

	return &Server{
		cfg:        cfg,
		authMW:     authMW,
		templates:  tmpl,
		dashboardH: dashboardH,
		tailscaleH: tailscaleH,
		caddyH:     caddyH,
		socatH:     socatH,
		backupH:    backupH,
		logsH:      logsH,
		staticFS:   staticFS,
		templateFS: templateFS,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Migrate existing Caddy proxies to metadata storage
	log.Printf("Migrating existing Caddy proxies to metadata storage...")
	if err := s.caddyH.MigrateExistingProxies(); err != nil {
		log.Printf("Warning: failed to migrate existing proxies: %v", err)
	}

	// Initialize autostart relays
	log.Printf("Initializing autostart relays...")
	if err := s.socatH.InitializeAutostart(); err != nil {
		log.Printf("Warning: failed to start autostart relays: %v", err)
	}

	// Start socat process monitor (checks every 10 seconds)
	log.Printf("Starting socat process monitor...")
	go s.socatH.StartProcessMonitor(s.ctx, 10*time.Second)

	// Initialize autostart proxies
	log.Printf("Initializing autostart proxies...")
	if err := s.caddyH.InitializeAutostart(); err != nil {
		log.Printf("Warning: failed to start autostart proxies: %v", err)
	}

	mux := s.setupRoutes()

	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	log.Printf("Starting Web UI server on %s", addr)

	// Create HTTP server for graceful shutdown support
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
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

		// Stop all socat relays
		log.Printf("Stopping all socat relays...")
		if err := s.socatH.StopAllRelays(); err != nil {
			log.Printf("Warning: failed to stop relays: %v", err)
		}

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

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Public routes (no authentication required)
	mux.HandleFunc("/login", s.handleLogin)
	mux.HandleFunc("/logout", s.handleLogout)
	mux.Handle("/api/tailscale/login", http.HandlerFunc(s.tailscaleH.Login))
	mux.Handle("/api/tailscale/poll", http.HandlerFunc(s.tailscaleH.PollStatus))

	// Static files with proper MIME types
	fileServer := http.FileServer(http.FS(s.staticFS))
	mux.Handle("/static/", http.StripPrefix("/static/", s.staticFileHandler(fileServer)))

	// Protected routes (authentication required)
	mux.Handle("/", s.authMW.RequireAuth(http.HandlerFunc(s.handleSPAFallback)))
	mux.Handle("/api/status", s.authMW.RequireAuth(http.HandlerFunc(s.dashboardH.APIStatus)))

	// Tailscale routes
	mux.Handle("/tailscale", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.Status)))
	mux.Handle("/api/tailscale/logout", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.Logout)))
	mux.Handle("/api/tailscale/connect", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.Connect)))
	mux.Handle("/api/tailscale/disconnect", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.Disconnect)))
	mux.Handle("/api/tailscale/status", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.APIStatus)))
	mux.Handle("/api/tailscale/peers", s.authMW.RequireAuth(http.HandlerFunc(s.tailscaleH.APIPeers)))

	// Caddy routes
	mux.Handle("/caddy", s.authMW.RequireAuth(http.HandlerFunc(s.handleSPARedirect)))
	mux.Handle("/api/caddy/create", s.authMW.RequireAuth(http.HandlerFunc(s.caddyH.Create)))
	mux.Handle("/api/caddy/update", s.authMW.RequireAuth(http.HandlerFunc(s.caddyH.Update)))
	mux.Handle("/api/caddy/delete", s.authMW.RequireAuth(http.HandlerFunc(s.caddyH.Delete)))
	mux.Handle("/api/caddy/toggle", s.authMW.RequireAuth(http.HandlerFunc(s.caddyH.Toggle)))
	mux.Handle("/api/caddy/reload", s.authMW.RequireAuth(http.HandlerFunc(s.caddyH.Reload)))
	mux.Handle("/api/caddy/proxies", s.authMW.RequireAuth(http.HandlerFunc(s.caddyH.APIList)))
	mux.Handle("/api/caddy/proxy", s.authMW.RequireAuth(http.HandlerFunc(s.caddyH.APIGet)))

	// Socat routes
	mux.Handle("/socat", s.authMW.RequireAuth(http.HandlerFunc(s.handleSPARedirect)))
	mux.Handle("/api/socat/create", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.Create)))
	mux.Handle("/api/socat/update", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.Update)))
	mux.Handle("/api/socat/delete", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.Delete)))
	mux.Handle("/api/socat/toggle", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.Toggle)))
	mux.Handle("/api/socat/start", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.Start)))
	mux.Handle("/api/socat/stop", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.Stop)))
	mux.Handle("/api/socat/restart", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.Restart)))
	mux.Handle("/api/socat/restart-all", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.RestartAll)))
	mux.Handle("/api/socat/relays", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.APIList)))
	mux.Handle("/api/socat/relay", s.authMW.RequireAuth(http.HandlerFunc(s.socatH.APIGet)))

	// Backup routes
	mux.Handle("/backup", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.List)))
	mux.Handle("/api/backup/create", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.Create)))
	mux.Handle("/api/backup/restore", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.Restore)))
	mux.Handle("/api/backup/delete", s.authMW.RequireAuth(http.HandlerFunc(s.backupH.Delete)))
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
func (s *Server) handleSPAFallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	if err := s.templates.ExecuteTemplate(w, "index.html", nil); err != nil {
		log.Printf("Error rendering SPA template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleSPARedirect sends legacy pages to the SPA.
func (s *Server) handleSPARedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogin handles the login page
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Show login instructions and tailscale login link
	s.templates.ExecuteTemplate(w, "login.html", nil)
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
