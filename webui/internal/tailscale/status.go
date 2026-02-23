package tailscale

import (
	"fmt"
	"strings"
	"time"
)

// StatusSummary provides a simplified view of Tailscale status
type StatusSummary struct {
	Connected    bool
	BackendState string
	Hostname     string
	MagicDNSName string
	IPv4         string
	IPv6         string
	TailnetName  string
	Version      string
	PeerCount    int
	ActivePeers  int
	Health       []string
	LastCheck    time.Time
}

// GetStatusSummary returns a simplified status summary
func (c *Client) GetStatusSummary() (*StatusSummary, error) {
	status, err := c.GetStatus()
	if err != nil {
		return &StatusSummary{
			Connected:    false,
			BackendState: "Unknown",
			LastCheck:    time.Now(),
		}, err
	}

	ipv4, ipv6, _ := c.GetIP()

	// Strip trailing dot from MagicDNS name (DNS FQDN format)
	magicDNS := strings.TrimSuffix(status.Self.DNSName, ".")

	summary := &StatusSummary{
		Connected:    status.BackendState == "Running",
		BackendState: status.BackendState,
		Hostname:     status.Self.HostName,
		MagicDNSName: magicDNS,
		IPv4:         ipv4,
		IPv6:         ipv6,
		Version:      status.Version,
		PeerCount:    len(status.Peer),
		Health:       status.Health,
		LastCheck:    time.Now(),
	}

	if status.CurrentTailnet != nil {
		summary.TailnetName = status.CurrentTailnet.Name
	}

	// Count active peers
	for _, peer := range status.Peer {
		if peer.Active && peer.Online {
			summary.ActivePeers++
		}
	}

	return summary, nil
}

// PeerInfo provides simplified peer information
type PeerInfo struct {
	Hostname string
	DNSName  string
	OS       string
	IPv4     string
	IPv6     string
	Active   bool
	Online   bool
	LastSeen time.Time
	Relay    string
	ExitNode bool
}

// GetPeers returns a list of peer information
func (c *Client) GetPeers() ([]PeerInfo, error) {
	status, err := c.GetStatus()
	if err != nil {
		return nil, err
	}

	peers := make([]PeerInfo, 0, len(status.Peer))
	for _, peer := range status.Peer {
		info := PeerInfo{
			Hostname: peer.HostName,
			DNSName:  strings.TrimSuffix(peer.DNSName, "."), // Strip trailing dot
			OS:       peer.OS,
			Active:   peer.Active,
			Online:   peer.Online,
			LastSeen: peer.LastSeen,
			Relay:    peer.Relay,
			ExitNode: peer.ExitNode,
		}

		// Extract IPv4 and IPv6
		for _, ip := range peer.TailscaleIPs {
			if len(ip) > 0 {
				if ip[0] == '1' { // IPv4 starts with 100.x.y.z
					info.IPv4 = ip
				} else if ip[0] == 'f' { // IPv6 starts with fd7a:
					info.IPv6 = ip
				}
			}
		}

		peers = append(peers, info)
	}

	return peers, nil
}

// HealthCheck checks if Tailscale is healthy
func (c *Client) HealthCheck() (bool, []string, error) {
	status, err := c.GetStatus()
	if err != nil {
		return false, nil, err
	}

	isHealthy := status.BackendState == "Running" && len(status.Health) == 0
	return isHealthy, status.Health, nil
}

// FormatBackendState returns a human-readable backend state
func FormatBackendState(state string) string {
	switch state {
	case "Running":
		return "Connected"
	case "Starting":
		return "Starting..."
	case "Stopped":
		return "Disconnected"
	case "NeedsLogin":
		return "Needs Login"
	case "NoState":
		return "Not Started"
	default:
		return state
	}
}

// FormatBytes formats bytes to human readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats a time duration to human readable format
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}
