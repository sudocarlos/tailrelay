package caddy

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
)

// LoadProxyMetadata loads proxy metadata from JSON file
func LoadProxyMetadata(filePath string) ([]config.CaddyProxy, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty list if file doesn't exist
			return []config.CaddyProxy{}, nil
		}
		return nil, fmt.Errorf("failed to read proxy metadata file: %w", err)
	}

	var proxyList config.CaddyProxyList
	if err := json.Unmarshal(data, &proxyList); err != nil {
		return nil, fmt.Errorf("failed to parse proxy metadata file: %w", err)
	}

	return proxyList.Proxies, nil
}

// SaveProxyMetadata saves proxy metadata to JSON file
func SaveProxyMetadata(filePath string, proxies []config.CaddyProxy) error {
	proxyList := config.CaddyProxyList{
		Proxies: proxies,
	}

	data, err := json.MarshalIndent(proxyList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal proxy metadata: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write proxy metadata file: %w", err)
	}

	return nil
}

// AddProxyMetadata adds a new proxy to the metadata file
func AddProxyMetadata(filePath string, proxy config.CaddyProxy) error {
	proxies, err := LoadProxyMetadata(filePath)
	if err != nil {
		return err
	}

	proxies = append(proxies, proxy)
	return SaveProxyMetadata(filePath, proxies)
}

// UpdateProxyMetadata updates an existing proxy in the metadata file
func UpdateProxyMetadata(filePath string, updatedProxy config.CaddyProxy) error {
	proxies, err := LoadProxyMetadata(filePath)
	if err != nil {
		return err
	}

	found := false
	for i, proxy := range proxies {
		if proxy.ID == updatedProxy.ID {
			proxies[i] = updatedProxy
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("proxy with ID %s not found", updatedProxy.ID)
	}

	return SaveProxyMetadata(filePath, proxies)
}

// DeleteProxyMetadata removes a proxy from the metadata file
func DeleteProxyMetadata(filePath string, proxyID string) error {
	proxies, err := LoadProxyMetadata(filePath)
	if err != nil {
		return err
	}

	newProxies := []config.CaddyProxy{}
	found := false
	for _, proxy := range proxies {
		if proxy.ID != proxyID {
			newProxies = append(newProxies, proxy)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("proxy with ID %s not found", proxyID)
	}

	return SaveProxyMetadata(filePath, newProxies)
}

// GetProxyMetadata retrieves a single proxy from the metadata file
func GetProxyMetadata(filePath string, proxyID string) (*config.CaddyProxy, error) {
	proxies, err := LoadProxyMetadata(filePath)
	if err != nil {
		return nil, err
	}

	for _, proxy := range proxies {
		if proxy.ID == proxyID {
			return &proxy, nil
		}
	}

	return nil, fmt.Errorf("proxy with ID %s not found", proxyID)
}
