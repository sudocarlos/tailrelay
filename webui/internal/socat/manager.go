package socat

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/sudocarlos/tailrelay-webui/internal/config"
	"github.com/sudocarlos/tailrelay-webui/internal/logger"
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
	logger.Debug("socat", "StartRelay called for relay %s (listen=%d, target=%s:%d)",
		relay.ID, relay.ListenPort, relay.TargetHost, relay.TargetPort)

	if !relay.Enabled {
		logger.Warn("socat", "Attempted to start disabled relay %s", relay.ID)
		return fmt.Errorf("relay is disabled")
	}

	// Check if already running
	if relay.PID != 0 && m.IsProcessRunning(relay.PID) {
		logger.Warn("socat", "Relay %s already running with PID %d", relay.ID, relay.PID)
		return fmt.Errorf("relay already running with PID %d", relay.PID)
	}

	// Build socat command
	// socat tcp-listen:PORT,fork,reuseaddr tcp:HOST:PORT
	listenAddr := fmt.Sprintf("tcp-listen:%d,fork,reuseaddr", relay.ListenPort)
	targetAddr := fmt.Sprintf("tcp:%s:%d", relay.TargetHost, relay.TargetPort)

	logger.Debug("socat", "Starting socat: %s %s %s", m.socatBinary, listenAddr, targetAddr)

	cmd := exec.Command(m.socatBinary, listenAddr, targetAddr)

	// Set process group ID to the process PID so we can kill the entire group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Start the process in background
	if err := cmd.Start(); err != nil {
		logger.Error("socat", "Failed to start relay %s on port %d: %v", relay.ID, relay.ListenPort, err)
		return fmt.Errorf("failed to start socat: %w", err)
	}

	// Update PID in relay config
	relay.PID = cmd.Process.Pid
	logger.Debug("socat", "Relay %s started with PID %d, updating config file", relay.ID, relay.PID)

	if err := UpdateRelayPID(m.relaysFile, relay.ID, relay.PID); err != nil {
		logger.Warn("socat", "Failed to update PID for relay %s in config: %v", relay.ID, err)
	}

	logger.Info("socat", "Started socat relay %s (PID %d): 0.0.0.0:%d -> %s:%d",
		relay.ID, relay.PID, relay.ListenPort, relay.TargetHost, relay.TargetPort)

	return nil
}

// StopRelay stops a running socat relay process
func (m *Manager) StopRelay(relay *config.SocatRelay) error {
	logger.Debug("socat", "StopRelay called for relay %s (PID=%d)", relay.ID, relay.PID)

	if relay.PID == 0 {
		logger.Warn("socat", "Cannot stop relay %s: no PID recorded", relay.ID)
		return fmt.Errorf("relay has no PID recorded")
	}

	// Kill the entire process group (socat uses fork)
	// Use negative PID to target the process group
	logger.Debug("socat", "Killing process group -%d (SIGTERM)", relay.PID)
	if err := syscall.Kill(-relay.PID, syscall.SIGTERM); err != nil {
		// If process group kill fails, try killing just the process
		logger.Debug("socat", "Process group kill failed, trying single process: %v", err)
		process, err := os.FindProcess(relay.PID)
		if err != nil {
			logger.Error("socat", "Failed to find process %d for relay %s: %v", relay.PID, relay.ID, err)
			return fmt.Errorf("failed to find process: %w", err)
		}

		logger.Debug("socat", "Sending SIGTERM to PID %d", relay.PID)
		if err := process.Signal(syscall.SIGTERM); err != nil {
			// Process might already be dead
			if err.Error() != "os: process already finished" {
				logger.Error("socat", "Failed to stop relay %s (PID %d): %v", relay.ID, relay.PID, err)
				return fmt.Errorf("failed to stop process: %w", err)
			}
			logger.Debug("socat", "Process %d already finished", relay.PID)
		}
	}

	// Wait a bit for processes to terminate gracefully
	// If SIGTERM doesn't work, send SIGKILL to process group
	logger.Debug("socat", "Waiting for process group to terminate...")
	for i := 0; i < 5; i++ {
		if !m.IsProcessRunning(relay.PID) {
			logger.Debug("socat", "Process %d terminated successfully", relay.PID)
			break
		}
		if i == 4 {
			// Last resort: SIGKILL to process group
			logger.Warn("socat", "Process %d did not terminate gracefully, sending SIGKILL to group", relay.PID)
			syscall.Kill(-relay.PID, syscall.SIGKILL)
		}
	}

	// Clear PID
	oldPID := relay.PID
	relay.PID = 0
	logger.Debug("socat", "Clearing PID for relay %s in config", relay.ID)

	if err := UpdateRelayPID(m.relaysFile, relay.ID, 0); err != nil {
		logger.Warn("socat", "Failed to clear PID for relay %s in config: %v", relay.ID, err)
	}

	logger.Info("socat", "Stopped socat relay %s (was PID %d)", relay.ID, oldPID)
	return nil
}

// RestartRelay restarts a relay
func (m *Manager) RestartRelay(relay *config.SocatRelay) error {
	logger.Debug("socat", "RestartRelay called for relay %s", relay.ID)

	// Stop if running
	if relay.PID != 0 {
		if err := m.StopRelay(relay); err != nil {
			logger.Warn("socat", "Failed to stop relay %s during restart: %v", relay.ID, err)
		}
	}

	// Start relay
	return m.StartRelay(relay)
}

// StartAll starts all relays with autostart enabled
func (m *Manager) StartAll() error {
	logger.Debug("socat", "StartAll: loading relays from %s", m.relaysFile)

	relays, err := LoadRelays(m.relaysFile)
	if err != nil {
		logger.Error("socat", "Failed to load relays from %s: %v", m.relaysFile, err)
		return fmt.Errorf("failed to load relays: %w", err)
	}

	started := 0
	failed := 0

	for i := range relays {
		// Only start relays with autostart enabled
		if !relays[i].Autostart {
			logger.Debug("socat", "Skipping relay %s (autostart disabled)", relays[i].ID)
			continue
		}

		// Enable the relay if autostart is on
		if !relays[i].Enabled {
			relays[i].Enabled = true
		}

		if err := m.StartRelay(&relays[i]); err != nil {
			logger.Error("socat", "Failed to start relay %s: %v", relays[i].ID, err)
			failed++
		} else {
			started++
		}
	}

	logger.Info("socat", "StartAll complete: %d started, %d failed", started, failed)
	return nil
}

// StopAll stops all running relays
func (m *Manager) StopAll() error {
	logger.Debug("socat", "StopAll: loading relays from %s", m.relaysFile)

	relays, err := LoadRelays(m.relaysFile)
	if err != nil {
		logger.Error("socat", "Failed to load relays from %s: %v", m.relaysFile, err)
		return fmt.Errorf("failed to load relays: %w", err)
	}

	stopped := 0
	failed := 0

	for i := range relays {
		if relays[i].PID == 0 {
			logger.Debug("socat", "Skipping relay %s (no PID)", relays[i].ID)
			continue
		}

		if err := m.StopRelay(&relays[i]); err != nil {
			logger.Error("socat", "Failed to stop relay %s: %v", relays[i].ID, err)
			failed++
		} else {
			stopped++
		}
	}

	logger.Info("socat", "StopAll complete: %d stopped, %d failed", stopped, failed)
	return nil
}

// RestartAll restarts all enabled relays
func (m *Manager) RestartAll() error {
	logger.Info("socat", "RestartAll: restarting all enabled relays")

	// Stop all first
	if err := m.StopAll(); err != nil {
		logger.Warn("socat", "Error stopping relays during RestartAll: %v", err)
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
