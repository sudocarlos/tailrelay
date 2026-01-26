package socat

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
)

// Manager handles socat process management
type Manager struct {
	socatBinary string
	relaysFile  string
}

// NewManager creates a new socat manager
func NewManager(socatBinary, relaysFile string) *Manager {
	if socatBinary == "" {
		socatBinary = "socat" // Default to PATH
	}

	return &Manager{
		socatBinary: socatBinary,
		relaysFile:  relaysFile,
	}
}

// StartRelay starts a single socat relay process
func (m *Manager) StartRelay(relay *config.SocatRelay) error {
	if !relay.Enabled {
		return fmt.Errorf("relay is disabled")
	}

	// Check if already running
	if relay.PID != 0 && m.IsProcessRunning(relay.PID) {
		return fmt.Errorf("relay already running with PID %d", relay.PID)
	}

	// Build socat command
	// socat tcp-listen:PORT,fork,reuseaddr tcp:HOST:PORT
	listenAddr := fmt.Sprintf("tcp-listen:%d,fork,reuseaddr", relay.ListenPort)
	targetAddr := fmt.Sprintf("tcp:%s:%d", relay.TargetHost, relay.TargetPort)

	cmd := exec.Command(m.socatBinary, listenAddr, targetAddr)

	// Start the process in background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start socat: %w", err)
	}

	// Update PID in relay config
	relay.PID = cmd.Process.Pid
	if err := UpdateRelayPID(m.relaysFile, relay.ID, relay.PID); err != nil {
		log.Printf("Warning: failed to update PID for relay %s: %v", relay.ID, err)
	}

	log.Printf("Started socat relay %s (PID %d): %s:%d -> %s:%d",
		relay.ID, relay.PID, "0.0.0.0", relay.ListenPort, relay.TargetHost, relay.TargetPort)

	return nil
}

// StopRelay stops a running socat relay process
func (m *Manager) StopRelay(relay *config.SocatRelay) error {
	if relay.PID == 0 {
		return fmt.Errorf("relay has no PID recorded")
	}

	// Send TERM signal to process
	process, err := os.FindProcess(relay.PID)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		if err.Error() != "os: process already finished" {
			return fmt.Errorf("failed to stop process: %w", err)
		}
	}

	// Clear PID
	relay.PID = 0
	if err := UpdateRelayPID(m.relaysFile, relay.ID, 0); err != nil {
		log.Printf("Warning: failed to clear PID for relay %s: %v", relay.ID, err)
	}

	log.Printf("Stopped socat relay %s", relay.ID)
	return nil
}

// RestartRelay restarts a relay
func (m *Manager) RestartRelay(relay *config.SocatRelay) error {
	// Stop if running
	if relay.PID != 0 {
		if err := m.StopRelay(relay); err != nil {
			log.Printf("Warning: failed to stop relay during restart: %v", err)
		}
	}

	// Start relay
	return m.StartRelay(relay)
}

// StartAll starts all enabled relays
func (m *Manager) StartAll() error {
	relays, err := LoadRelays(m.relaysFile)
	if err != nil {
		return fmt.Errorf("failed to load relays: %w", err)
	}

	started := 0
	failed := 0

	for i := range relays {
		if !relays[i].Enabled {
			continue
		}

		if err := m.StartRelay(&relays[i]); err != nil {
			log.Printf("Failed to start relay %s: %v", relays[i].ID, err)
			failed++
		} else {
			started++
		}
	}

	log.Printf("Started %d socat relays (%d failed)", started, failed)
	return nil
}

// StopAll stops all running relays
func (m *Manager) StopAll() error {
	relays, err := LoadRelays(m.relaysFile)
	if err != nil {
		return fmt.Errorf("failed to load relays: %w", err)
	}

	stopped := 0
	failed := 0

	for i := range relays {
		if relays[i].PID == 0 {
			continue
		}

		if err := m.StopRelay(&relays[i]); err != nil {
			log.Printf("Failed to stop relay %s: %v", relays[i].ID, err)
			failed++
		} else {
			stopped++
		}
	}

	log.Printf("Stopped %d socat relays (%d failed)", stopped, failed)
	return nil
}

// RestartAll restarts all enabled relays
func (m *Manager) RestartAll() error {
	// Stop all first
	if err := m.StopAll(); err != nil {
		log.Printf("Warning: error stopping relays: %v", err)
	}

	// Start all enabled
	return m.StartAll()
}

// IsProcessRunning checks if a process with given PID is running
func (m *Manager) IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// GetStatus returns status of all relays
func (m *Manager) GetStatus() ([]RelayStatus, error) {
	relays, err := LoadRelays(m.relaysFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load relays: %w", err)
	}

	statuses := make([]RelayStatus, len(relays))
	for i, relay := range relays {
		running := false
		if relay.PID != 0 {
			running = m.IsProcessRunning(relay.PID)

			// Clear stale PID if process is not running
			if !running && relay.PID != 0 {
				UpdateRelayPID(m.relaysFile, relay.ID, 0)
			}
		}

		statuses[i] = RelayStatus{
			Relay:   relay,
			Running: running,
		}
	}

	return statuses, nil
}

// RelayStatus represents the status of a relay
type RelayStatus struct {
	Relay   config.SocatRelay
	Running bool
}
