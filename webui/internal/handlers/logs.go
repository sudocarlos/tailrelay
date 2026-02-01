package handlers

import (
"encoding/json"
"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/sudocarlos/tailrelay-webui/internal/logger"
)

// Handler is a base handler with templates
type Handler struct {
	templates *template.Template
}

// NewHandler creates a new base handler
func NewHandler(templates *template.Template) *Handler {
	return &Handler{
		templates: templates,
	}
}

// LogsPageHandler serves the logs viewing page
func (h *Handler) LogsPageHandler(w http.ResponseWriter, r *http.Request) {
data := map[string]interface{}{
"Title":       "Logs",
"CurrentPage": "logs",
"LogLevel":    logger.Get().GetLevelName(),
}

if err := h.templates.ExecuteTemplate(w, "logs.html", data); err != nil {
http.Error(w, err.Error(), http.StatusInternalServerError)
}
}

// LogsAPIHandler returns historical logs as JSON
func (h *Handler) LogsAPIHandler(w http.ResponseWriter, r *http.Request) {
logs := logger.Get().GetHistory()

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]interface{}{
"logs":  logs,
"level": logger.Get().GetLevelName(),
})
}

// LogsStreamHandler streams logs via Server-Sent Events
func (h *Handler) LogsStreamHandler(w http.ResponseWriter, r *http.Request) {
// Set headers for SSE
w.Header().Set("Content-Type", "text/event-stream")
w.Header().Set("Cache-Control", "no-cache")
w.Header().Set("Connection", "keep-alive")
w.Header().Set("Access-Control-Allow-Origin", "*")

// Subscribe to log events
logChan := logger.Get().Subscribe()
defer logger.Get().Unsubscribe(logChan)

// Create flusher
flusher, ok := w.(http.Flusher)
if !ok {
http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
return
}

// Send initial connection message
fmt.Fprintf(w, "data: {\"connected\": true}\n\n")
flusher.Flush()

// Stream logs
for {
select {
case <-r.Context().Done():
return
case entry, ok := <-logChan:
if !ok {
return
}

// Serialize log entry to JSON
data, err := json.Marshal(entry)
if err != nil {
continue
}

// Send SSE event
fmt.Fprintf(w, "data: %s\n\n", data)
flusher.Flush()
}
}
}

// LogsLevelHandler gets or sets the log level
func (h *Handler) LogsLevelHandler(w http.ResponseWriter, r *http.Request) {
switch r.Method {
case http.MethodGet:
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]interface{}{
"level": logger.Get().GetLevelName(),
})

case http.MethodPost:
var req struct {
Level string `json:"level"`
}

if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
http.Error(w, "Invalid request", http.StatusBadRequest)
return
}

level, err := logger.ParseLevel(req.Level)
if err != nil {
http.Error(w, fmt.Sprintf("Invalid log level: %s", req.Level), http.StatusBadRequest)
return
}

logger.Get().SetLevel(level)
logger.Info("logs", "Log level changed to %s by user %s", strings.ToUpper(req.Level), r.RemoteAddr)

w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w).Encode(map[string]interface{}{
"success": true,
"level":   logger.Get().GetLevelName(),
})

default:
http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
}
