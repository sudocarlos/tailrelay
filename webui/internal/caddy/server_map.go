package caddy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ServerMap stores mappings between proxy identifiers and Caddy server names.
type ServerMap struct {
	ByProxyID  map[string]string `json:"by_proxy_id"`
	ByHostPort map[string]string `json:"by_host_port"`
	NextIndex  int               `json:"next_index"`
}

func NewServerMap() *ServerMap {
	return &ServerMap{
		ByProxyID:  make(map[string]string),
		ByHostPort: make(map[string]string),
		NextIndex:  0,
	}
}

func LoadServerMap(filePath string) (*ServerMap, error) {
	if filePath == "" {
		return NewServerMap(), nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewServerMap(), nil
		}
		return nil, fmt.Errorf("read server map: %w", err)
	}

	var m ServerMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal server map: %w", err)
	}

	if m.ByProxyID == nil {
		m.ByProxyID = make(map[string]string)
	}
	if m.ByHostPort == nil {
		m.ByHostPort = make(map[string]string)
	}

	return &m, nil
}

func SaveServerMap(filePath string, m *ServerMap) error {
	if filePath == "" {
		return nil
	}

	if m == nil {
		return fmt.Errorf("server map is nil")
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal server map: %w", err)
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create server map dir: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write server map: %w", err)
	}

	return nil
}
