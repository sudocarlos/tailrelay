package tailscale

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
	"time"
)

// Status represents the output of 'tailscale status --json'
type Status struct {
	Version        string                `json:"Version"`
	BackendState   string                `json:"BackendState"`
	Self           PeerStatus            `json:"Self"`
	Health         []string              `json:"Health"`
	MagicDNSSuffix string                `json:"MagicDNSSuffix"`
	CurrentTailnet *CurrentTailnet       `json:"CurrentTailnet"`
	Peer           map[string]PeerStatus `json:"Peer"`
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

// GetStatus returns the current Tailscale status
func (c *Client) GetStatus() (*Status, error) {
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

// IsConnected checks if Tailscale is connected
func (c *Client) IsConnected() (bool, error) {
	status, err := c.GetStatus()
	if err != nil {
		return false, err
	}

	return status.BackendState == "Running", nil
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

// Login initiates a Tailscale login and returns the auth URL
func (c *Client) Login() (string, error) {
	// Use tailscale up to trigger login
	cmd := exec.Command(c.binaryPath, "up", "--auth-key=", "--timeout=1s")
	output, _ := cmd.CombinedOutput()

	// Parse output for auth URL
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "http") && strings.Contains(line, "login") {
			// Extract URL from line
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "http") {
					return strings.TrimSpace(part), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no auth URL found in output")
}

// LoginWithQR initiates login and returns auth URL with QR code flag
func (c *Client) LoginWithQR() (string, error) {
	cmd := exec.Command(c.binaryPath, "up", "--qr", "--timeout=5s")
	output, _ := cmd.CombinedOutput()

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "http") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "http") {
					return strings.TrimSpace(part), nil
				}
			}
		}
	}

	return "", fmt.Errorf("no auth URL found")
}

// Logout logs out from Tailscale
func (c *Client) Logout() error {
	cmd := exec.Command(c.binaryPath, "logout")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	return nil
}

// Down disconnects from Tailscale
func (c *Client) Down() error {
	cmd := exec.Command(c.binaryPath, "down")
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

// Netcheck runs a network check
func (c *Client) Netcheck() (string, error) {
	cmd := exec.Command(c.binaryPath, "netcheck")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run netcheck: %w", err)
	}

	return string(output), nil
}
