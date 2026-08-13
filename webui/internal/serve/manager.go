package serve

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"sort"
	"strings"
	"sync"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/logger"
)

// Manager manages relay rules backed by `tailscale serve`.
type Manager struct {
	binaryPath           string
	relayFile            string
	customControlServer  bool
	controlServerChecker controlServerChecker
	mu                   sync.RWMutex
}

// controlServerChecker reports whether tailscaled is currently authenticated
// against a control server other than Tailscale's default (e.g. a
// self-hosted Headscale instance). Implemented by *tailscale.Client in
// production; kept as an interface here so tests can fake it without a
// dependency cycle or shelling out to a real tailscaled.
type controlServerChecker interface {
	IsCustomControlServer() (bool, error)
}

// NewManager creates a new serve manager.
func NewManager(relayFile string) *Manager {
	return &Manager{
		binaryPath: "tailscale",
		relayFile:  relayFile,
	}
}

// NewManagerWithCustomControlServer creates a manager that always uses HTTP
// web listeners (customControlServer=true) or HTTPS (false), with no live
// detection. Intended for tests; production code should use
// NewManagerWithControlServerDetection so the scheme reflects tailscaled's
// actual control server even if it drifts from persisted config.
func NewManagerWithCustomControlServer(relayFile string, customControlServer bool) *Manager {
	m := NewManager(relayFile)
	m.customControlServer = customControlServer
	return m
}

// NewManagerWithControlServerDetection creates a manager that determines the
// web listener scheme (--https vs --http) from tailscaled's live ControlURL
// preference via checker, since custom control servers (e.g. Headscale) do
// not provide Tailscale's HTTPS certificate provisioning and reject --https
// serve requests outright.
//
// fallback is used only when checker is nil or the live check fails (e.g.
// tailscaled isn't reachable yet during startup) — it should be seeded from
// the persisted Web UI control server setting so relays still reconcile
// sensibly before the daemon is up.
func NewManagerWithControlServerDetection(relayFile string, checker controlServerChecker, fallback bool) *Manager {
	m := NewManager(relayFile)
	m.controlServerChecker = checker
	m.customControlServer = fallback
	return m
}

// NewManagerWithBinary creates a new serve manager that invokes binaryPath
// instead of the real `tailscale` CLI. Intended for tests that need to stub
// out CLI behavior (e.g. handler-level tests in other packages); production
// code should use NewManager.
func NewManagerWithBinary(relayFile, binaryPath string) *Manager {
	m := NewManager(relayFile)
	m.binaryPath = binaryPath
	return m
}

// WebListenerScheme returns the effective scheme for web relays, preferring
// a live check of tailscaled's actual control server over the persisted
// fallback flag so it stays correct even if the node was authenticated
// outside the Web UI (CLI login, restored state, etc.).
//
// This performs a LocalAPI round-trip (via the configured checker) on every
// call, so it must not be called per-relay inside a loop — callers resolve
// it once per request/reconcile and reuse the result (see resetAndApply and
// handlers.ServeHandler.APIListHTTPS).
func (m *Manager) WebListenerScheme() string {
	m.mu.RLock()
	checker := m.controlServerChecker
	fallback := m.customControlServer
	m.mu.RUnlock()

	custom := fallback
	if checker != nil {
		if live, err := checker.IsCustomControlServer(); err == nil {
			custom = live
		} else {
			logger.Debug("serve", "Falling back to persisted control server setting, live check failed: %v", err)
		}
	}
	if custom {
		return "http"
	}
	return "https"
}

// SetCustomControlServer updates the fallback web listener mode used when
// live control server detection is unavailable (no checker configured, or
// the last live check failed).
func (m *Manager) SetCustomControlServer(customControlServer bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.customControlServer = customControlServer
}

// ServeStatusJSON is the structure returned by `tailscale serve status --json`
type ServeStatusJSON struct {
	TCP map[string]struct {
		HTTPS      bool   `json:"HTTPS,omitempty"`
		TCPForward string `json:"TCPForward,omitempty"`
	} `json:"TCP"`
	Web map[string]struct {
		Handlers map[string]struct {
			Proxy string `json:"Proxy,omitempty"`
		} `json:"Handlers"`
	} `json:"Web"`
	// AllowFunnel maps "host:port" to whether that port is exposed to the
	// public internet via `tailscale funnel`.
	AllowFunnel map[string]bool `json:"AllowFunnel,omitempty"`
}

// FunnelPorts are the only ports `tailscale funnel` is permitted to listen
// on, per Tailscale's Funnel documentation.
var FunnelPorts = []int{443, 8443, 10000}

// IsFunnelPort reports whether port is one of the allowed funnel ports.
func IsFunnelPort(port int) bool {
	for _, p := range FunnelPorts {
		if p == port {
			return true
		}
	}
	return false
}

// Status returns the parsed output of `tailscale serve status --json`
func (m *Manager) Status() (*ServeStatusJSON, error) {
	cmd := exec.Command(m.binaryPath, "serve", "status", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tailscale serve status failed: %w", err)
	}
	var status ServeStatusJSON
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// RelayStatus is a UI status shape similar to legacy relay status APIs.
type RelayStatus struct {
	Relay   config.ServeRelay `json:"relay"`
	Running bool              `json:"running"`
}

// ListRelays returns all stored relay definitions.
func (m *Manager) ListRelays() ([]config.ServeRelay, error) {
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		return nil, err
	}
	return list.Relays, nil
}

// GetRelay gets a relay by ID.
func (m *Manager) GetRelay(id string) (*config.ServeRelay, error) {
	relays, err := m.ListRelays()
	if err != nil {
		return nil, err
	}
	for i := range relays {
		if relays[i].ID == id {
			return &relays[i], nil
		}
	}
	return nil, fmt.Errorf("relay not found")
}

// UpsertRelay creates or updates a relay and reconciles tailscale serve state.
func (m *Manager) UpsertRelay(relay config.ServeRelay) error {
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		return err
	}

	if relay.Type == "" {
		relay.Type = "tcp"
	}
	relay.Type = strings.ToLower(strings.TrimSpace(relay.Type))
	if relay.Type != "https" && relay.Type != "tcp" && relay.Type != "funnel" {
		return fmt.Errorf("relay type must be https, tcp, or funnel")
	}

	if relay.ID == "" {
		relay.ID = fmt.Sprintf("%s-%d", relay.Type, relay.ListenPort)
	}

	if relay.Type == "funnel" {
		relay.FunnelTransport = strings.ToLower(strings.TrimSpace(relay.FunnelTransport))
		if relay.FunnelTransport == "" {
			relay.FunnelTransport = "https"
		}
		if relay.FunnelTransport != "https" && relay.FunnelTransport != "tcp" {
			return fmt.Errorf("funnel_transport must be https or tcp")
		}
		if !IsFunnelPort(relay.ListenPort) {
			return fmt.Errorf("funnel listen_port must be one of %v", FunnelPorts)
		}
	} else if relay.ListenPort < 1 || relay.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be between 1 and 65535")
	}
	if relay.TargetPort < 1 || relay.TargetPort > 65535 {
		return fmt.Errorf("target_port must be between 1 and 65535")
	}
	if strings.TrimSpace(relay.TargetHost) == "" {
		return fmt.Errorf("target_host is required")
	}
	relay.IconURL = strings.TrimSpace(relay.IconURL)
	if err := ValidateIconURL(relay.IconURL); err != nil {
		return err
	}

	updated := false
	for i := range list.Relays {
		if list.Relays[i].ID == relay.ID {
			list.Relays[i] = relay
			updated = true
			break
		}
	}
	if !updated {
		list.Relays = append(list.Relays, relay)
	}

	if err := config.SaveServeRelays(m.relayFile, list); err != nil {
		return err
	}
	return m.Reconcile()
}

// DeleteRelay removes a relay and reconciles tailscale serve state.
func (m *Manager) DeleteRelay(id string) error {
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		return err
	}
	filtered := make([]config.ServeRelay, 0, len(list.Relays))
	found := false
	for _, r := range list.Relays {
		if r.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}
	if !found {
		return ErrRelayNotFound
	}
	list.Relays = filtered
	if err := config.SaveServeRelays(m.relayFile, list); err != nil {
		return err
	}
	return m.Reconcile()
}

// ToggleRelay toggles a relay enabled state and reconciles tailscale serve state.
func (m *Manager) ToggleRelay(id string, enabled bool) error {
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		return err
	}
	found := false
	for i := range list.Relays {
		if list.Relays[i].ID == id {
			list.Relays[i].Enabled = enabled
			found = true
			break
		}
	}
	if !found {
		return ErrRelayNotFound
	}
	if err := config.SaveServeRelays(m.relayFile, list); err != nil {
		return err
	}
	return m.Reconcile()
}

// ErrTailscaleNotReady is returned by Reconcile when Tailscale is not yet
// authenticated or connected. Callers can use errors.Is to distinguish a
// deferred reconcile from a real failure.
var ErrTailscaleNotReady = fmt.Errorf("tailscale not ready")

// ErrRelayNotFound is returned by DeleteRelay and ToggleRelay when no relay
// with the given ID exists in the config file.
var ErrRelayNotFound = fmt.Errorf("relay not found")

// ErrFunnelNotAllowed is returned when `tailscale funnel` refuses to apply a
// relay because the tailnet policy file is missing the `funnel` node
// attribute for this device, or Funnel is otherwise disabled for the tailnet.
var ErrFunnelNotAllowed = fmt.Errorf("funnel not allowed")

// ErrInvalidIconURL is returned by UpsertRelay when IconURL is set but is
// neither an http/https URL nor a small data:image/* URI. Handlers map this to
// 400 Bad Request via writeServeResult.
var ErrInvalidIconURL = fmt.Errorf("invalid icon_url")

// MaxIconDataLen caps the length of a data: URI icon so a large base64 blob
// can't bloat serve_relays.json. http(s) URLs are not capped (they reference an
// external resource fetched by the browser).
const MaxIconDataLen = 256 << 10 // 256 KiB

// ValidateIconURL returns ErrInvalidIconURL (wrapping a reason) if iconURL is
// set but is neither an http/https URL nor a small data:image/* URI. An empty
// value is valid (no icon configured).
func ValidateIconURL(iconURL string) error {
	iconURL = strings.TrimSpace(iconURL)
	if iconURL == "" {
		return nil
	}
	lower := strings.ToLower(iconURL)
	if strings.HasPrefix(lower, "data:") {
		if !strings.HasPrefix(lower, "data:image/") {
			return fmt.Errorf("%w: data URI must be an image MIME type", ErrInvalidIconURL)
		}
		if len(iconURL) > MaxIconDataLen {
			return fmt.Errorf("%w: data URI exceeds %d bytes", ErrInvalidIconURL, MaxIconDataLen)
		}
		return nil
	}
	u, err := url.Parse(iconURL)
	if err != nil {
		return fmt.Errorf("%w: not a valid URL: %v", ErrInvalidIconURL, err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidIconURL)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: must have a host", ErrInvalidIconURL)
	}
	return nil
}

// isFunnelNotAllowed reports whether err indicates the tailnet policy file is
// missing the `funnel` node attribute (or Funnel is otherwise unavailable),
// as opposed to a generic command failure.
func isFunnelNotAllowed(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "funnel is not enabled") ||
		strings.Contains(s, "funnel attribute") ||
		strings.Contains(s, "not allowed to run funnel") ||
		strings.Contains(s, "requires funnel")
}

// isTailscaleNotReady reports whether err indicates Tailscale is not yet
// authenticated or connected (e.g. during container startup in CI).
func isTailscaleNotReady(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "netMap is nil") ||
		strings.Contains(s, "not logged in") ||
		strings.Contains(s, "NeedsLogin") ||
		strings.Contains(s, "connect: no such file or directory")
}

// Reconcile resets tailscale serve config and reapplies all enabled relays.
// If Tailscale is not yet authenticated or connected the reconcile is skipped
// with a warning — relay config is already persisted and will be applied on
// the next successful reconcile (e.g. after Tailscale authenticates).
func (m *Manager) Reconcile() error {
	logger.Debug("serve", "Starting serve reconcile process")
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		logger.Error("serve", "Failed to load serve relays: %v", err)
		return err
	}

	active := make([]config.ServeRelay, 0, len(list.Relays))
	for _, r := range list.Relays {
		if r.Enabled {
			active = append(active, r)
		}
	}
	return m.resetAndApply(active)
}

// ReconcileAutostart resets tailscale serve config and reapplies all relays
// that have Autostart=true. Intended to be called once at container startup so
// that relays marked for autostart are brought up regardless of their last
// runtime Enabled state.
// If Tailscale is not yet authenticated or connected the reconcile is skipped —
// relay config is already persisted and will be applied on the next successful
// reconcile (e.g. after Tailscale authenticates).
func (m *Manager) ReconcileAutostart() error {
	logger.Debug("serve", "Starting autostart reconcile process")
	list, err := config.LoadServeRelays(m.relayFile)
	if err != nil {
		logger.Error("serve", "Failed to load serve relays: %v", err)
		return err
	}

	active := make([]config.ServeRelay, 0, len(list.Relays))
	for _, r := range list.Relays {
		if r.Autostart {
			active = append(active, r)
		}
	}
	return m.resetAndApply(active)
}

// resetAndApply resets the tailscale serve config and applies the given relays.
func (m *Manager) resetAndApply(relays []config.ServeRelay) error {
	logger.Debug("serve", "Resetting tailscale serve config")
	if err := m.run("serve", "reset"); err != nil {
		if isTailscaleNotReady(err) {
			logger.Debug("serve", "Tailscale not ready, deferring reconcile: %v", err)
			return ErrTailscaleNotReady
		}
		logger.Error("serve", "tailscale serve reset failed: %v", err)
		return err
	}

	sort.Slice(relays, func(i, j int) bool {
		if relays[i].ListenPort == relays[j].ListenPort {
			return relays[i].ID < relays[j].ID
		}
		return relays[i].ListenPort < relays[j].ListenPort
	})

	// Resolve once per reconcile: WebListenerScheme() may perform a live
	// LocalAPI prefs lookup, so it must not be called per-relay below.
	webScheme := m.WebListenerScheme()

	for _, relay := range relays {
		logger.Debug("serve", "Applying relay %s (type: %s, port: %d)", relay.ID, relay.Type, relay.ListenPort)
		if err := m.applyRelay(relay, webScheme); err != nil {
			if isTailscaleNotReady(err) {
				logger.Debug("serve", "Tailscale not ready, deferring reconcile: %v", err)
				return ErrTailscaleNotReady
			}
			if relay.Type == "funnel" && isFunnelNotAllowed(err) {
				logger.Error("serve", "Funnel not allowed for relay %s: %v", relay.ID, err)
				return ErrFunnelNotAllowed
			}
			logger.Error("serve", "Failed to apply relay %s: %v", relay.ID, err)
			return err
		}
	}
	logger.Debug("serve", "Finished reconcile process")
	return nil
}

// applyRelay applies relay via `tailscale serve`/`tailscale funnel`. webScheme
// is the already-resolved WebListenerScheme() result for this reconcile pass
// (see resetAndApply), threaded through instead of re-resolved per relay.
func (m *Manager) applyRelay(relay config.ServeRelay, webScheme string) error {
	target := ""
	switch relay.Type {
	case "https":
		protocol := "http"
		if relay.TargetHTTPS {
			protocol = "https+insecure"
		}
		target = fmt.Sprintf("%s://%s:%d", protocol, relay.TargetHost, relay.TargetPort)
		return m.run("serve", "--bg", "--"+webScheme, fmt.Sprintf("%d", relay.ListenPort), target)
	case "tcp":
		target = fmt.Sprintf("tcp://%s:%d", relay.TargetHost, relay.TargetPort)
		return m.run("serve", "--bg", "--tcp", fmt.Sprintf("%d", relay.ListenPort), target)
	case "funnel":
		return m.applyFunnel(relay)
	default:
		return fmt.Errorf("unsupported relay type: %s", relay.Type)
	}
}

// applyFunnel applies a funnel relay via `tailscale funnel`, translating the
// stored FunnelTransport ("https" or "tcp") into the matching CLI flag.
func (m *Manager) applyFunnel(relay config.ServeRelay) error {
	switch relay.FunnelTransport {
	case "tcp":
		target := fmt.Sprintf("tcp://%s:%d", relay.TargetHost, relay.TargetPort)
		return m.run("funnel", "--bg", "--tcp", fmt.Sprintf("%d", relay.ListenPort), target)
	case "https", "":
		protocol := "http"
		if relay.TargetHTTPS {
			protocol = "https+insecure"
		}
		target := fmt.Sprintf("%s://%s:%d", protocol, relay.TargetHost, relay.TargetPort)
		return m.run("funnel", "--bg", "--https", fmt.Sprintf("%d", relay.ListenPort), target)
	default:
		return fmt.Errorf("unsupported funnel transport: %s", relay.FunnelTransport)
	}
}

func (m *Manager) run(args ...string) error {
	cmdStr := fmt.Sprintf("%s %s", m.binaryPath, strings.Join(args, " "))
	logger.Debug("serve", "Executing: %s", cmdStr)

	cmd := exec.Command(m.binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errStr := fmt.Sprintf("tailscale %s failed: %s (output: %s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		if strings.Contains(errStr, "netMap is nil") {
			logger.Debug("serve", "Tailscale not fully ready yet: %v", errStr)
		} else {
			logger.Error("serve", "%s", errStr)
		}
		return fmt.Errorf("%s", errStr)
	}
	logger.Debug("serve", "Execution successful: %s", cmdStr)
	return nil
}
