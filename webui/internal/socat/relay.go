package socat

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
)

// LoadRelays loads relay configurations from JSON file
func LoadRelays(filePath string) ([]config.SocatRelay, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty list if file doesn't exist
			return []config.SocatRelay{}, nil
		}
		return nil, fmt.Errorf("failed to read relays file: %w", err)
	}

	var relayList config.SocatRelayList
	if err := json.Unmarshal(data, &relayList); err != nil {
		return nil, fmt.Errorf("failed to parse relays file: %w", err)
	}

	return relayList.Relays, nil
}

// SaveRelays saves relay configurations to JSON file
func SaveRelays(filePath string, relays []config.SocatRelay) error {
	relayList := config.SocatRelayList{
		Relays: relays,
	}

	data, err := json.MarshalIndent(relayList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal relays: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write relays file: %w", err)
	}

	return nil
}

// AddRelay adds a new relay to the list
func AddRelay(filePath string, relay config.SocatRelay) error {
	relays, err := LoadRelays(filePath)
	if err != nil {
		return err
	}

	relays = append(relays, relay)

	return SaveRelays(filePath, relays)
}

// UpdateRelay updates an existing relay by ID
func UpdateRelay(filePath string, updatedRelay config.SocatRelay) error {
	relays, err := LoadRelays(filePath)
	if err != nil {
		return err
	}

	found := false
	for i, relay := range relays {
		if relay.ID == updatedRelay.ID {
			// Preserve PID if running
			if relays[i].PID != 0 {
				updatedRelay.PID = relays[i].PID
			}
			relays[i] = updatedRelay
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("relay with ID %s not found", updatedRelay.ID)
	}

	return SaveRelays(filePath, relays)
}

// DeleteRelay removes a relay by ID
func DeleteRelay(filePath string, relayID string) error {
	relays, err := LoadRelays(filePath)
	if err != nil {
		return err
	}

	newRelays := []config.SocatRelay{}
	found := false
	for _, relay := range relays {
		if relay.ID != relayID {
			newRelays = append(newRelays, relay)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("relay with ID %s not found", relayID)
	}

	return SaveRelays(filePath, newRelays)
}

// ToggleRelay enables or disables a relay by ID
func ToggleRelay(filePath string, relayID string, enabled bool) error {
	relays, err := LoadRelays(filePath)
	if err != nil {
		return err
	}

	found := false
	for i, relay := range relays {
		if relay.ID == relayID {
			relays[i].Enabled = enabled
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("relay with ID %s not found", relayID)
	}

	return SaveRelays(filePath, relays)
}

// GetRelay retrieves a single relay by ID
func GetRelay(filePath string, relayID string) (*config.SocatRelay, error) {
	relays, err := LoadRelays(filePath)
	if err != nil {
		return nil, err
	}

	for _, relay := range relays {
		if relay.ID == relayID {
			return &relay, nil
		}
	}

	return nil, fmt.Errorf("relay with ID %s not found", relayID)
}

// UpdateRelayPID updates the PID for a relay
func UpdateRelayPID(filePath string, relayID string, pid int) error {
	relays, err := LoadRelays(filePath)
	if err != nil {
		return err
	}

	found := false
	for i, relay := range relays {
		if relay.ID == relayID {
			relays[i].PID = pid
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("relay with ID %s not found", relayID)
	}

	return SaveRelays(filePath, relays)
}
