package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sudocarlos/tailrelay-webui/internal/caddy"
	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/tailscale"
)

// CaddyHandler handles Caddy-related requests
type CaddyHandler struct {
	cfg       *config.Config
	templates *template.Template
	manager   *caddy.Manager
	tsClient  *tailscale.Client
}

// NewCaddyHandler creates a new Caddy handler
func NewCaddyHandler(cfg *config.Config, templates *template.Template) *CaddyHandler {
	// Use Caddy API instead of file-based config
	// Pass empty string for server name to enable auto-discovery
	manager := caddy.NewManager(
		caddy.DefaultAdminAPI,
		cfg.Paths.CaddyServerMap,
	)

	return &CaddyHandler{
		cfg:       cfg,
		templates: templates,
		manager:   manager,
		tsClient:  tailscale.NewClient(),
	}
}

// MigrateExistingProxies migrates existing Caddy proxies to metadata storage
func (h *CaddyHandler) MigrateExistingProxies() error {
	return h.manager.MigrateExistingProxies()
}

// InitializeAutostart starts all proxies with autostart enabled
func (h *CaddyHandler) InitializeAutostart() error {
	return h.manager.InitializeAutostart()
}

// List renders the Caddy proxy management page
func (h *CaddyHandler) List(w http.ResponseWriter, r *http.Request) {
	proxies, err := h.manager.ListProxies()
	if err != nil {
		log.Printf("Error loading proxies: %v", err)
		proxies = []config.CaddyProxy{}
	}

	// Get Caddy status
	running, _ := h.manager.GetStatus()

	// Get Tailscale FQDN
	tailscaleFQDN := ""
	if status, err := h.tsClient.GetStatusSummary(); err == nil {
		tailscaleFQDN = status.MagicDNSName
	}

	// Count enabled proxies
	enabledCount := 0
	for _, proxy := range proxies {
		if proxy.Enabled {
			enabledCount++
		}
	}

	data := map[string]interface{}{
		"Title":         "Caddy Proxies",
		"Proxies":       proxies,
		"Running":       running,
		"EnabledCount":  enabledCount,
		"TailscaleFQDN": tailscaleFQDN,
	}

	if err := h.templates.ExecuteTemplate(w, "caddy.html", data); err != nil {
		log.Printf("Error rendering caddy template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create handles creating a new proxy
func (h *CaddyHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxy, err := h.parseProxyFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	proxy.Hostname = caddy.NormalizeHostname(proxy.Hostname)

	// Set default enabled state
	if !proxy.Enabled {
		proxy.Enabled = true
	}

	// Add proxy via API (no reload needed - API handles it instantly)
	createdProxy, err := h.manager.AddProxy(proxy)
	if err != nil {
		log.Printf("Error adding proxy: %v", err)
		http.Error(w, "Failed to add proxy", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Proxy created successfully",
		"proxy":   createdProxy,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Update handles updating an existing proxy
func (h *CaddyHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxy, err := h.parseProxyFromRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if proxy.ID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}

	proxy.Hostname = caddy.NormalizeHostname(proxy.Hostname)

	// Update proxy via API (no reload needed - API handles it instantly)
	if err := h.manager.UpdateProxy(proxy); err != nil {
		log.Printf("Error updating proxy: %v", err)
		http.Error(w, "Failed to update proxy", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":  "success",
		"message": "Proxy updated successfully",
		"proxy":   proxy,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Delete handles deleting a proxy
func (h *CaddyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	proxyID := r.URL.Query().Get("id")
	if proxyID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}

	// Delete proxy via API (no reload needed - API handles it instantly)
	if err := h.manager.DeleteProxy(proxyID); err != nil {
		log.Printf("Error deleting proxy: %v", err)
		http.Error(w, "Failed to delete proxy", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Proxy deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Toggle handles enabling/disabling a proxy
func (h *CaddyHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.ID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}

	// Toggle proxy via API (no reload needed - API handles it instantly)
	if err := h.manager.ToggleProxy(request.ID, request.Enabled); err != nil {
		log.Printf("Error toggling proxy: %v", err)
		http.Error(w, "Failed to toggle proxy", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Proxy toggled successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Reload handles reloading Caddy configuration
// Note: This is now a no-op since Caddy API handles changes instantly
// Kept for backwards compatibility with the Web UI
func (h *CaddyHandler) Reload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if Caddy API is accessible
	running, err := h.manager.GetStatus()
	if err != nil || !running {
		log.Printf("Error checking Caddy status: %v", err)
		http.Error(w, fmt.Sprintf("Caddy API not accessible: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Caddy configuration is up to date (API-based management)",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIList returns all proxies as JSON
func (h *CaddyHandler) APIList(w http.ResponseWriter, r *http.Request) {
	proxies, err := h.manager.ListProxies()
	if err != nil {
		log.Printf("Error loading proxies: %v", err)
		http.Error(w, "Failed to load proxies", http.StatusInternalServerError)
		return
	}

	running, _ := h.manager.GetStatus()

	response := make([]struct {
		config.CaddyProxy
		Running bool `json:"running"`
	}, 0, len(proxies))

	for _, proxy := range proxies {
		response = append(response, struct {
			config.CaddyProxy
			Running bool `json:"running"`
		}{
			CaddyProxy: proxy,
			Running:    running,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIGet returns a single proxy as JSON
func (h *CaddyHandler) APIGet(w http.ResponseWriter, r *http.Request) {
	proxyID := r.URL.Query().Get("id")
	if proxyID == "" {
		http.Error(w, "Proxy ID is required", http.StatusBadRequest)
		return
	}

	proxy, err := h.manager.GetProxy(proxyID)
	if err != nil {
		log.Printf("Error getting proxy: %v", err)
		http.Error(w, "Proxy not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxy)
}

func (h *CaddyHandler) parseProxyFromRequest(r *http.Request) (config.CaddyProxy, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return h.parseProxyFromMultipart(r)
	}

	var proxy config.CaddyProxy
	if err := json.NewDecoder(r.Body).Decode(&proxy); err != nil {
		return config.CaddyProxy{}, fmt.Errorf("invalid request body")
	}

	return proxy, nil
}

func (h *CaddyHandler) parseProxyFromMultipart(r *http.Request) (config.CaddyProxy, error) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		return config.CaddyProxy{}, fmt.Errorf("failed to parse form data")
	}

	proxy := config.CaddyProxy{}
	proxy.ID = r.FormValue("id")
	proxy.Hostname = r.FormValue("hostname")
	proxy.Target = r.FormValue("target")
	proxy.TLSCertFile = r.FormValue("tls_cert_file")

	if portStr := r.FormValue("port"); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return config.CaddyProxy{}, fmt.Errorf("invalid port")
		}
		proxy.Port = port
	}

	proxy.Enabled = parseBool(r.FormValue("enabled"))
	proxy.TrustedProxies = parseBool(r.FormValue("trusted_proxies"))
	proxy.TLS = parseBool(r.FormValue("tls"))

	file, fileHeader, err := r.FormFile("tls_cert_upload")
	if err == nil {
		defer file.Close()
		certPath, err := h.saveTLSCertFile(proxy.Target, file, fileHeader)
		if err != nil {
			return config.CaddyProxy{}, err
		}
		proxy.TLSCertFile = certPath
	}

	return proxy, nil
}

func (h *CaddyHandler) saveTLSCertFile(target string, file multipart.File, header *multipart.FileHeader) (string, error) {
	if target == "" {
		return "", fmt.Errorf("target is required for cert upload")
	}

	parsed, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("invalid target URL")
	}

	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	if host == "" {
		return "", fmt.Errorf("invalid target host")
	}

	nameBase := sanitizeName(host)
	fileName := fmt.Sprintf("%s-%s.cert", nameBase, port)

	certDir := h.cfg.Paths.CertificatesDir
	if certDir == "" {
		certDir = "/data"
	}

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return "", fmt.Errorf("create cert dir: %w", err)
	}

	fullPath := filepath.Join(certDir, fileName)
	fullPath = ensureUniqueFile(fullPath)

	out, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("create cert file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		return "", fmt.Errorf("write cert file: %w", err)
	}

	if header != nil && header.Filename != "" {
		_ = header
	}

	return fullPath, nil
}

func sanitizeName(input string) string {
	name := strings.ToLower(strings.TrimSpace(input))
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, ":", "-")
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

func ensureUniqueFile(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	base := strings.TrimSuffix(path, filepath.Ext(path))
	ext := filepath.Ext(path)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

func parseBool(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "true" || value == "1" || value == "on" || value == "yes"
}
