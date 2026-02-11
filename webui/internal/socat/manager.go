package socat

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

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

	// If relay has a PID, check if it's actually running
	// If not, clear the stale PID
	if relay.PID != 0 {
		isRunning := m.IsProcessRunning(relay.PID)
		logger.Info("socat", "Relay %s has PID %d, checking if running: %v", relay.ID, relay.PID, isRunning)
		if isRunning {
			logger.Warn("socat", "Relay %s already running with PID %d", relay.ID, relay.PID)
			return fmt.Errorf("relay already running with PID %d", relay.PID)
		}
		// Clear stale PID
		logger.Info("socat", "Clearing stale PID %d for relay %s before starting", relay.PID, relay.ID)
		relay.PID = 0
		if err := UpdateRelayPID(m.relaysFile, relay.ID, 0); err != nil {
			logger.Warn("socat", "Failed to clear stale PID: %v", err)
		}
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
		logger.Debug("socat", "Relay %s has no PID - already stopped", relay.ID)
		return nil // Idempotent: already stopped
	}

	// Check if process is already dead before attempting to kill
	if !m.IsProcessRunning(relay.PID) {
		logger.Debug("socat", "Process %d for relay %s is already dead, clearing PID", relay.PID, relay.ID)
		relay.PID = 0
		if err := UpdateRelayPID(m.relaysFile, relay.ID, 0); err != nil {
			logger.Warn("socat", "Failed to clear PID for relay %s in config: %v", relay.ID, err)
		}
		return nil // Idempotent: process was already dead
	}

	// Kill the entire process group (socat uses fork)
	// Use negative PID to target the process group
	logger.Debug("socat", "Killing process group -%d (SIGTERM)", relay.PID)
	if err := syscall.Kill(-relay.PID, syscall.SIGTERM); err != nil {
		// If process group kill fails, try killing just the process
		logger.Debug("socat", "Process group kill failed (%v), trying single process", err)
		process, err := os.FindProcess(relay.PID)
		if err != nil {
			logger.Debug("socat", "Failed to find process %d for relay %s: %v (likely already dead)", relay.PID, relay.ID, err)
			// Process is gone - treat as success
			relay.PID = 0
			if err := UpdateRelayPID(m.relaysFile, relay.ID, 0); err != nil {
				logger.Warn("socat", "Failed to clear PID for relay %s in config: %v", relay.ID, err)
			}
			return nil
		}

		logger.Debug("socat", "Sending SIGTERM to PID %d", relay.PID)
		if err := process.Signal(syscall.SIGTERM); err != nil {
			// Process might already be dead - check the error
			logger.Debug("socat", "Failed to signal process %d: %v (likely already dead)", relay.PID, err)
			// Verify the process is actually gone
			if !m.IsProcessRunning(relay.PID) {
				logger.Debug("socat", "Confirmed process %d is not running", relay.PID)
				relay.PID = 0
				if err := UpdateRelayPID(m.relaysFile, relay.ID, 0); err != nil {
					logger.Warn("socat", "Failed to clear PID for relay %s in config: %v", relay.ID, err)
				}
				return nil
			}
			// Process exists but we couldn't signal it - this is a real error
			logger.Error("socat", "Failed to stop relay %s (PID %d): %v", relay.ID, relay.PID, err)
			return fmt.Errorf("failed to stop process: %w", err)
		}
	}

	// Wait for processes to terminate gracefully
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
			if err := syscall.Kill(-relay.PID, syscall.SIGKILL); err != nil {
				logger.Debug("socat", "SIGKILL to process group failed: %v (process may already be dead)", err)
				// Try single process SIGKILL
				if err := syscall.Kill(relay.PID, syscall.SIGKILL); err != nil {
					logger.Debug("socat", "SIGKILL to process failed: %v", err)
				}
			}
		}
		// Wait 200ms between checks (total ~1 second for graceful shutdown)
		time.Sleep(200 * time.Millisecond)
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

	// First pass: clear any stale PIDs from previous runs
	logger.Info("socat", "Checking for stale PIDs before starting autostart relays...")
	staleCleaned := 0
	for i := range relays {
		if relays[i].PID != 0 && !m.IsProcessRunning(relays[i].PID) {
			logger.Info("socat", "Clearing stale PID %d for relay %s", relays[i].PID, relays[i].ID)
			if err := UpdateRelayPID(m.relaysFile, relays[i].ID, 0); err != nil {
				logger.Warn("socat", "Failed to clear stale PID for relay %s: %v", relays[i].ID, err)
			} else {
				relays[i].PID = 0
				staleCleaned++
			}
		}
	}
	if staleCleaned > 0 {
		logger.Info("socat", "Cleared %d stale PID(s)", staleCleaned)
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
// and is actually a socat process (not a zombie, thread, or unrelated process)
func (m *Manager) IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Check if process/thread exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}

	// Check /proc/PID/status to verify:
	// 1. It's actually a socat process (Name field)
	// 2. It's a process, not a thread (Tgid == Pid)
	// 3. It's not a zombie (State field)
	statusPath := fmt.Sprintf("/proc/%d/status", pid)
	statusData, err := os.ReadFile(statusPath)
	if err != nil {
		// /proc entry doesn't exist or can't be read - process is gone
		return false
	}

	var name, state string
	var tgid, pidVal int
	lines := strings.Split(string(statusData), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Name:\t") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "Name:\t"))
		} else if strings.HasPrefix(line, "State:\t") {
			state = strings.TrimSpace(strings.TrimPrefix(line, "State:\t"))
		} else if strings.HasPrefix(line, "Tgid:\t") {
			fmt.Sscanf(line, "Tgid:\t%d", &tgid)
		} else if strings.HasPrefix(line, "Pid:\t") {
			fmt.Sscanf(line, "Pid:\t%d", &pidVal)
		}
	}

	// Check if it's a thread (Tgid != Pid means it's a thread)
	if tgid != pidVal {
		logger.Debug("socat", "PID %d is a thread (Tgid=%d), not a process", pid, tgid)
		return false
	}

	// Check if it's a zombie process (State starts with 'Z')
	if strings.HasPrefix(state, "Z") {
		logger.Debug("socat", "PID %d is a zombie process", pid)
		return false
	}

	// Check if it's actually socat
	if name != "socat" {
		logger.Debug("socat", "PID %d is '%s', not socat", pid, name)
		return false
	}

	return true
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

// MonitorProcesses periodically checks for dead processes and cleans up stale PIDs
// This runs in a background goroutine and should be called with a context that
// can be cancelled when the application shuts down.
func (m *Manager) MonitorProcesses(ctx context.Context, interval time.Duration) {
	logger.Info("socat", "Starting process monitor (interval: %v)", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("socat", "Process monitor shutting down")
			return
		case <-ticker.C:
			m.checkAndCleanDeadProcesses()
		}
	}
}

// checkAndCleanDeadProcesses checks all relay processes and clears stale PIDs
func (m *Manager) checkAndCleanDeadProcesses() {
	relays, err := LoadRelays(m.relaysFile)
	if err != nil {
		logger.Warn("socat", "Monitor: failed to load relays: %v", err)
		return
	}

	cleanedCount := 0
	for i := range relays {
		if relays[i].PID == 0 {
			continue // No PID to check
		}

		if !m.IsProcessRunning(relays[i].PID) {
			logger.Info("socat", "Monitor: detected dead process for relay %s (PID %d), cleaning up", relays[i].ID, relays[i].PID)
			if err := UpdateRelayPID(m.relaysFile, relays[i].ID, 0); err != nil {
				logger.Warn("socat", "Monitor: failed to clear PID for relay %s: %v", relays[i].ID, err)
			} else {
				cleanedCount++
			}
		}
	}

	if cleanedCount > 0 {
		logger.Info("socat", "Monitor: cleaned up %d dead process(es)", cleanedCount)
	}
}
