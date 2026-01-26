package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sudocarlos/tailrelay-webui/internal/backup"
	"github.com/sudocarlos/tailrelay-webui/internal/config"
)

// BackupHandler handles backup-related requests
type BackupHandler struct {
	cfg       *config.Config
	templates *template.Template
	manager   *backup.Manager
}

// NewBackupHandler creates a new backup handler
func NewBackupHandler(cfg *config.Config, templates *template.Template) *BackupHandler {
	manager := backup.NewManager(cfg)

	return &BackupHandler{
		cfg:       cfg,
		templates: templates,
		manager:   manager,
	}
}

// List renders the backup management page
func (h *BackupHandler) List(w http.ResponseWriter, r *http.Request) {
	backups, err := h.manager.List()
	if err != nil {
		log.Printf("Error loading backups: %v", err)
		backups = []config.BackupInfo{}
	}

	data := map[string]interface{}{
		"Title":   "Backup & Restore",
		"Backups": backups,
	}

	if err := h.templates.ExecuteTemplate(w, "backup.html", data); err != nil {
		log.Printf("Error rendering backup template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// Create handles creating a new backup
func (h *BackupHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		BackupType string `json:"backup_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		// Default to full backup if no type specified
		request.BackupType = "full"
	}

	if request.BackupType == "" {
		request.BackupType = "full"
	}

	backupPath, err := h.manager.Create(request.BackupType)
	if err != nil {
		log.Printf("Error creating backup: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create backup: %v", err), http.StatusInternalServerError)
		return
	}

	// Cleanup old backups
	if h.cfg.Backup.RetentionCount > 0 {
		if err := h.manager.CleanupOldBackups(h.cfg.Backup.RetentionCount); err != nil {
			log.Printf("Warning: failed to cleanup old backups: %v", err)
		}
	}

	response := map[string]interface{}{
		"status":      "success",
		"message":     "Backup created successfully",
		"backup_path": backupPath,
		"filename":    filepath.Base(backupPath),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Restore handles restoring from a backup
func (h *BackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Filename string `json:"filename"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.Filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	backupPath := filepath.Join(h.cfg.Paths.BackupDir, request.Filename)

	if err := h.manager.Restore(backupPath); err != nil {
		log.Printf("Error restoring backup: %v", err)
		http.Error(w, fmt.Sprintf("Failed to restore backup: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Backup restored successfully. Please restart services for changes to take effect.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Delete handles deleting a backup
func (h *BackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	if err := h.manager.Delete(filename); err != nil {
		log.Printf("Error deleting backup: %v", err)
		http.Error(w, fmt.Sprintf("Failed to delete backup: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": "Backup deleted successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Download handles downloading a backup file
func (h *BackupHandler) Download(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Filename is required", http.StatusBadRequest)
		return
	}

	backupPath := filepath.Join(h.cfg.Paths.BackupDir, filename)

	// Security check: ensure the file is in the backup directory
	absBackupPath, err := filepath.Abs(backupPath)
	if err != nil {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	absBackupDir, err := filepath.Abs(h.cfg.Paths.BackupDir)
	if err != nil {
		http.Error(w, "Invalid backup directory", http.StatusInternalServerError)
		return
	}

	if len(absBackupPath) < len(absBackupDir) || absBackupPath[:len(absBackupDir)] != absBackupDir {
		http.Error(w, "Invalid backup path", http.StatusBadRequest)
		return
	}

	// Check if file exists
	info, err := os.Stat(backupPath)
	if err != nil {
		http.Error(w, "Backup not found", http.StatusNotFound)
		return
	}

	// Open file
	file, err := os.Open(backupPath)
	if err != nil {
		log.Printf("Error opening backup file: %v", err)
		http.Error(w, "Failed to open backup", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set headers for download
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	// Stream file to response
	http.ServeContent(w, r, filename, info.ModTime(), file)
}

// Upload handles uploading a backup file
func (h *BackupHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (32MB max)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("backup")
	if err != nil {
		http.Error(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate filename
	if !strings.HasSuffix(handler.Filename, ".tar.gz") {
		http.Error(w, "Invalid file type, must be .tar.gz", http.StatusBadRequest)
		return
	}

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(h.cfg.Paths.BackupDir, 0755); err != nil {
		log.Printf("Error creating backup directory: %v", err)
		http.Error(w, "Failed to create backup directory", http.StatusInternalServerError)
		return
	}

	// Save file
	backupPath := filepath.Join(h.cfg.Paths.BackupDir, handler.Filename)
	dst, err := os.Create(backupPath)
	if err != nil {
		log.Printf("Error creating backup file: %v", err)
		http.Error(w, "Failed to create backup file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := file.Seek(0, 0); err != nil {
		log.Printf("Error seeking file: %v", err)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	if _, err := dst.ReadFrom(file); err != nil {
		log.Printf("Error saving backup: %v", err)
		http.Error(w, "Failed to save backup", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":   "success",
		"message":  "Backup uploaded successfully",
		"filename": handler.Filename,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIList returns all backups as JSON
func (h *BackupHandler) APIList(w http.ResponseWriter, r *http.Request) {
	backups, err := h.manager.List()
	if err != nil {
		log.Printf("Error loading backups: %v", err)
		http.Error(w, "Failed to load backups", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(backups)
}
