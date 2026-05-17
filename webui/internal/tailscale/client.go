package tailscale

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os/exec"
	"strings"
	"time"
)

// localAPISocketPath is the default path for the tailscaled Unix socket.
const localAPISocketPath = "/var/run/tailscale/tailscaled.sock"

// localAPIStatus is a minimal representation of the LocalAPI /status response.
// We only decode the fields we care about.
type localAPIStatus struct {
	BackendState string `json:"BackendState"`
	AuthURL      string `json:"AuthURL"`
}

// newLocalAPIClient returns an http.Client that speaks to the tailscaled
// Unix socket. Inside the container everything runs as root, so no extra
// authentication is needed.
func newLocalAPIClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", localAPISocketPath)
			},
		},
		Timeout: 10 * time.Second,
	}
}

// localAPIGet performs a GET against the tailscaled LocalAPI.
func localAPIGet(path string) ([]byte, int, error) {
	client := newLocalAPIClient()
	resp, err := client.Get("http://local-tailscaled.sock" + path)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// localAPIPost performs a POST against the tailscaled LocalAPI.
func localAPIPost(path string) ([]byte, int, error) {
	client := newLocalAPIClient()
	resp, err := client.Post("http://local-tailscaled.sock"+path, "application/json", nil)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

// Status represents the output of 'tailscale status --json'
type Status struct {
	Version        string                `json:"Version"`
	BackendState   string                `json:"BackendState"`
	Self           PeerStatus            `json:"Self"`
	Health         []string              `json:"Health"`
	MagicDNSSuffix string                `json:"MagicDNSSuffix"`
	CurrentTailnet *CurrentTailnet       `json:"CurrentTailnet"`
	Peer           map[string]PeerStatus `json:"Peer"`
	User           map[string]UserProfile `json:"User"`
}

// UserProfile represents a Tailscale user
type UserProfile struct {
	ID            int64  `json:"ID"`
	LoginName     string `json:"LoginName"`
	DisplayName   string `json:"DisplayName"`
	ProfilePicURL string `json:"ProfilePicURL"`
}

// PeerStatus represents a peer device
type PeerStatus struct {
	ID             string    `json:"ID"`
	HostName       string    `json:"HostName"`
	DNSName        string    `json:"DNSName"`
	OS             string    `json:"OS"`
	UserID         int       `json:"UserID"`
	TailscaleIPs   []string  `json:"TailscaleIPs"`
	Tags           []string  `json:"Tags,omitempty"`
	PrimaryRoutes  []string  `json:"PrimaryRoutes,omitempty"`
	Active         bool      `json:"Active"`
	ExitNode       bool      `json:"ExitNode,omitempty"`
	ExitNodeOption bool      `json:"ExitNodeOption,omitempty"`
	Online         bool      `json:"Online"`
	Relay          string    `json:"Relay,omitempty"`
	RxBytes        int64     `json:"RxBytes,omitempty"`
	TxBytes        int64     `json:"TxBytes,omitempty"`
	Created        time.Time `json:"Created"`
	LastSeen       time.Time `json:"LastSeen"`
	LastHandshake  time.Time `json:"LastHandshake,omitempty"`
}

// CurrentTailnet represents the current tailnet information
type CurrentTailnet struct {
	Name            string `json:"Name"`
	MagicDNSSuffix  string `json:"MagicDNSSuffix"`
	MagicDNSEnabled bool   `json:"MagicDNSEnabled"`
}

// Client is a wrapper for the tailscale CLI
type Client struct {
	binaryPath string
}

// NewClient creates a new Tailscale client
func NewClient() *Client {
	return &Client{
		binaryPath: "tailscale",
	}
}

// GetStatus returns the current Tailscale status via the LocalAPI.
// Falls back to the CLI ('tailscale status --json') if the socket is unavailable.
func (c *Client) GetStatus() (*Status, error) {
	body, code, err := localAPIGet("/localapi/v0/status")
	if err == nil && code == http.StatusOK {
		var status Status
		if jsonErr := json.Unmarshal(body, &status); jsonErr == nil {
			return &status, nil
		}
	}

	// Fallback: CLI
	cmd := exec.Command(c.binaryPath, "status", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	var status Status
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, fmt.Errorf("failed to parse status: %w", err)
	}

	return &status, nil
}

// IsConnected checks if Tailscale is connected via the LocalAPI.
func (c *Client) IsConnected() (bool, error) {
	body, code, err := localAPIGet("/localapi/v0/status")
	if err != nil {
		return false, fmt.Errorf("failed to reach tailscaled: %w", err)
	}
	if code != http.StatusOK {
		return false, fmt.Errorf("tailscaled status returned %d", code)
	}
	var s localAPIStatus
	if err := json.Unmarshal(body, &s); err != nil {
		return false, fmt.Errorf("failed to parse status: %w", err)
	}
	return s.BackendState == "Running", nil
}

// GetIP returns the Tailscale IP addresses
func (c *Client) GetIP() (ipv4, ipv6 string, err error) {
	cmd := exec.Command(c.binaryPath, "ip")
	output, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to get IP: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		ip, err := netip.ParseAddr(strings.TrimSpace(line))
		if err != nil {
			continue
		}

		if ip.Is4() {
			ipv4 = ip.String()
		} else if ip.Is6() {
			ipv6 = ip.String()
		}
	}

	return ipv4, ipv6, nil
}

// Login triggers the Tailscale interactive login flow via the LocalAPI and
// returns the auth URL the user must visit to authenticate this device.
//
// It calls POST /localapi/v0/login-interactive to start the flow, then polls
// GET /localapi/v0/status until AuthURL is populated (up to 10 seconds).
func (c *Client) Login() (string, error) {
	// Trigger the login flow. This requires the process to be running as root
	// (or the configured operator), which is always the case inside the container.
	body, code, err := localAPIPost("/localapi/v0/login-interactive")
	if err != nil {
		return "", fmt.Errorf("failed to start login flow: %w", err)
	}
	if code != http.StatusOK && code != http.StatusNoContent {
		return "", fmt.Errorf("login-interactive returned %d: %s", code, strings.TrimSpace(string(body)))
	}

	// Poll status for AuthURL — the daemon generates it asynchronously.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)

		statusBody, _, err := localAPIGet("/localapi/v0/status")
		if err != nil {
			continue
		}
		var s localAPIStatus
		if err := json.Unmarshal(statusBody, &s); err != nil {
			continue
		}
		if s.AuthURL != "" {
			return s.AuthURL, nil
		}
	}

	return "", fmt.Errorf("timed out waiting for Tailscale auth URL")
}

// parseAuthURL extracts a Tailscale auth URL from command output.
// Used as a fallback for CLI-based commands that print URLs to stdout/stderr.
func parseAuthURL(output string) (string, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, "https://"); idx >= 0 {
			url := line[idx:]
			if end := strings.IndexAny(url, " \t\r\n"); end >= 0 {
				url = url[:end]
			}
			return url, nil
		}
	}
	return "", fmt.Errorf("no auth URL found in tailscale output")
}

// Logout logs out from Tailscale
func (c *Client) Logout() error {
	cmd := exec.Command(c.binaryPath, "logout", "--reason=Logout by Tailrelay")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	return nil
}

// Down disconnects from Tailscale
func (c *Client) Down() error {
	cmd := exec.Command(c.binaryPath, "down", "--reason=Disconnected by Tailrelay")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to disconnect: %w", err)
	}
	return nil
}

// Up connects to Tailscale
func (c *Client) Up() error {
	cmd := exec.Command(c.binaryPath, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	return nil
}

// UpWithHostname connects to Tailscale with a specific hostname.
// This runs 'tailscale up --hostname=<hostname>' which updates the device
// name in the tailnet. Note: this will re-run the Tailscale connection
// process and may require re-authentication if the node is not yet connected.
func (c *Client) UpWithHostname(hostname string) error {
	if hostname == "" {
		return fmt.Errorf("hostname cannot be empty")
	}
	cmd := exec.Command(c.binaryPath, "up", "--hostname="+hostname, "--reset")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set hostname: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// GetVersion returns the Tailscale version
func (c *Client) GetVersion() (string, error) {
	cmd := exec.Command(c.binaryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	version := strings.Split(strings.TrimSpace(string(output)), "\n")[0]
	return version, nil
}

// LoginWithAuthKey authenticates this device into a tailnet using a pre-generated
// auth key (e.g. "tskey-auth-k..."). This is a non-interactive alternative to
// Login(): instead of returning a URL for the user to visit, the key is passed
// directly to `tailscale up --authkey=<key>`, which authenticates and connects
// in a single step.
func (c *Client) LoginWithAuthKey(key string) error {
	if key == "" {
		return fmt.Errorf("auth key cannot be empty")
	}
	cmd := exec.Command(c.binaryPath, "up", "--authkey="+key)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to authenticate with auth key: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// Netcheck runs a network check
func (c *Client) Netcheck() (string, error) {
	cmd := exec.Command(c.binaryPath, "netcheck")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run netcheck: %w", err)
	}

	return string(output), nil
}
