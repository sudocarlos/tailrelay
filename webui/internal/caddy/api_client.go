package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sudocarlos/tailrelay-webui/internal/logger"
)

const (
	// DefaultAdminAPI is the default Caddy admin API address
	DefaultAdminAPI = "http://localhost:2019"
)

// APIClient provides methods to interact with Caddy's admin API
type APIClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewAPIClient creates a new Caddy API client
func NewAPIClient(baseURL string) *APIClient {
	if baseURL == "" {
		baseURL = DefaultAdminAPI
	}

	return &APIClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// doRequest performs an HTTP request and returns the response body
func (c *APIClient) doRequest(method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	var bodyPreview string

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			logger.Error("caddy", "Failed to marshal request body: %v", err)
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(data)
		// Preview first 200 chars for logging
		if len(data) > 200 {
			bodyPreview = string(data[:200]) + "..."
		} else {
			bodyPreview = string(data)
		}
	}

	url := c.BaseURL + path
	logger.Debug("caddy", "Caddy API request: %s %s", method, url)
	if bodyPreview != "" {
		logger.Debug("caddy", "Request body: %s", bodyPreview)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		logger.Error("caddy", "Failed to create HTTP request for %s %s: %v", method, url, err)
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		logger.Error("caddy", "HTTP request failed for %s %s: %v", method, url, err)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("caddy", "Failed to read response body from %s %s: %v", method, url, err)
		return nil, fmt.Errorf("read response: %w", err)
	}

	// Log response status and body preview
	respPreview := string(respBody)
	if len(respPreview) > 200 {
		respPreview = respPreview[:200] + "..."
	}
	logger.Debug("caddy", "Caddy API response: %d %s", resp.StatusCode, resp.Status)
	if len(respBody) > 0 {
		logger.Debug("caddy", "Response body: %s", respPreview)
	}

	if resp.StatusCode >= 400 {
		logger.Error("caddy", "Caddy API error %d for %s %s: %s", resp.StatusCode, method, url, string(respBody))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// GetConfig retrieves the entire Caddy configuration
func (c *APIClient) GetConfig(path string) (json.RawMessage, error) {
	if path == "" {
		path = "/"
	}
	data, err := c.doRequest("GET", "/config"+path, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// PostConfig adds or appends to configuration at the specified path
// For arrays, this appends. For objects, this creates or replaces.
func (c *APIClient) PostConfig(path string, config interface{}) error {
	_, err := c.doRequest("POST", "/config"+path, config)
	return err
}

// PatchConfig replaces configuration at the specified path
// This strictly replaces an existing value or array element
func (c *APIClient) PatchConfig(path string, config interface{}) error {
	_, err := c.doRequest("PATCH", "/config"+path, config)
	return err
}

// PutConfig inserts configuration at the specified path
// For arrays, this inserts. For objects, it strictly creates a new value.
func (c *APIClient) PutConfig(path string, config interface{}) error {
	_, err := c.doRequest("PUT", "/config"+path, config)
	return err
}

// DeleteConfig removes configuration at the specified path
func (c *APIClient) DeleteConfig(path string) error {
	_, err := c.doRequest("DELETE", "/config"+path, nil)
	return err
}

// GetByID retrieves configuration by @id tag
func (c *APIClient) GetByID(id string) (json.RawMessage, error) {
	data, err := c.doRequest("GET", "/id/"+id, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// PatchByID updates configuration by @id tag
func (c *APIClient) PatchByID(id string, config interface{}) error {
	_, err := c.doRequest("PATCH", "/id/"+id, config)
	return err
}

// DeleteByID removes configuration by @id tag
func (c *APIClient) DeleteByID(id string) error {
	_, err := c.doRequest("DELETE", "/id/"+id, nil)
	return err
}

// LoadConfig loads a complete configuration (replaces entire config)
func (c *APIClient) LoadConfig(config interface{}) error {
	_, err := c.doRequest("POST", "/load", config)
	return err
}

// GetReverseProxyUpstreams returns the status of all reverse proxy upstreams
func (c *APIClient) GetReverseProxyUpstreams() ([]UpstreamStatus, error) {
	data, err := c.doRequest("GET", "/reverse_proxy/upstreams", nil)
	if err != nil {
		return nil, err
	}

	var upstreams []UpstreamStatus
	if err := json.Unmarshal(data, &upstreams); err != nil {
		return nil, fmt.Errorf("unmarshal upstreams: %w", err)
	}

	return upstreams, nil
}

// DiscoverServerName discovers the first HTTP server name from Caddy config
// Returns the first server name found, or empty string if none exist
func (c *APIClient) DiscoverServerName() (string, error) {
	data, err := c.GetConfig("/apps/http/servers")
	if err != nil {
		return "", fmt.Errorf("get servers: %w", err)
	}

	// Parse as map to get server names
	var servers map[string]interface{}
	if err := json.Unmarshal(data, &servers); err != nil {
		return "", fmt.Errorf("unmarshal servers: %w", err)
	}

	// Return first server name found
	for name := range servers {
		logger.Debug("caddy", "Discovered Caddy server name: %s", name)
		return name, nil
	}

	return "", fmt.Errorf("no HTTP servers found in Caddy config")
}

// UpstreamStatus represents the status of a reverse proxy upstream
type UpstreamStatus struct {
	Address     string `json:"address"`
	NumRequests int    `json:"num_requests"`
	Fails       int    `json:"fails"`
}
