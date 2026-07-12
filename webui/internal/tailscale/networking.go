package tailscale

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// exitNodeRouteV4 and exitNodeRouteV6 are the special CIDRs that Tailscale
// uses internally to implement exit node advertisement. They ride on the
// same AdvertiseRoutes preference as ordinary subnet routes: advertising
// both together is what `--advertise-exit-node` actually does under the hood.
const (
	exitNodeRouteV4 = "0.0.0.0/0"
	exitNodeRouteV6 = "::/0"
)

// Prefs is a subset of tailscaled's ipn.Prefs, decoded from the LocalAPI
// '/localapi/v0/prefs' response. Only the fields the Web UI's Networking
// section cares about are represented here.
type Prefs struct {
	RouteAll               bool     `json:"RouteAll"`
	ExitNodeID             string   `json:"ExitNodeID"`
	ExitNodeIP             string   `json:"ExitNodeIP"`
	ExitNodeAllowLANAccess bool     `json:"ExitNodeAllowLANAccess"`
	RunSSH                 bool     `json:"RunSSH"`
	AdvertiseRoutes        []string `json:"AdvertiseRoutes"`
}

// GetPrefs returns the current tailscaled preferences via the LocalAPI.
func (c *Client) GetPrefs() (*Prefs, error) {
	body, code, err := localAPIGet("/localapi/v0/prefs")
	if err != nil {
		return nil, fmt.Errorf("failed to reach tailscaled: %w", err)
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("prefs endpoint returned %d", code)
	}

	var prefs Prefs
	if err := json.Unmarshal(body, &prefs); err != nil {
		return nil, fmt.Errorf("failed to parse prefs: %w", err)
	}
	return &prefs, nil
}

// NetworkingSummary is a simplified view of the networking-related Tailscale
// preferences surfaced in the Web UI's Networking section.
type NetworkingSummary struct {
	AdvertiseExitNode      bool
	ExitNodeAllowLANAccess bool
	AdvertiseRoutes        []string // custom subnet routes only; exit-node CIDRs are excluded
	AcceptRoutes           bool
	ExitNode               string
	SSH                    bool
}

// GetNetworkingSummary returns the current networking preferences in a form
// convenient for the frontend.
func (c *Client) GetNetworkingSummary() (*NetworkingSummary, error) {
	prefs, err := c.GetPrefs()
	if err != nil {
		return nil, err
	}
	return summarizeNetworking(prefs), nil
}

// summarizeNetworking derives a NetworkingSummary from raw Prefs.
// AdvertiseExitNode and the exit-node CIDRs are derived from AdvertiseRoutes
// since Tailscale has no separate preference for exit-node advertisement —
// it is implemented as the pair of default routes 0.0.0.0/0 and ::/0 within
// AdvertiseRoutes. Split out from GetNetworkingSummary so this derivation
// logic can be unit tested without a running tailscaled.
func summarizeNetworking(prefs *Prefs) *NetworkingSummary {
	summary := &NetworkingSummary{
		ExitNodeAllowLANAccess: prefs.ExitNodeAllowLANAccess,
		AcceptRoutes:           prefs.RouteAll,
		ExitNode:               prefs.ExitNodeIP,
		SSH:                    prefs.RunSSH,
	}

	hasV4, hasV6 := false, false
	for _, route := range prefs.AdvertiseRoutes {
		switch route {
		case exitNodeRouteV4:
			hasV4 = true
		case exitNodeRouteV6:
			hasV6 = true
		default:
			summary.AdvertiseRoutes = append(summary.AdvertiseRoutes, route)
		}
	}
	summary.AdvertiseExitNode = hasV4 && hasV6

	return summary
}

// NetworkingOptions represents a partial update to the networking
// preferences managed by the Networking section, applied via `tailscale
// set`. A nil field is left unchanged; only non-nil fields are passed as
// flags, matching `tailscale set`'s own "only what you specify changes"
// semantics.
type NetworkingOptions struct {
	AdvertiseExitNode      *bool
	ExitNodeAllowLANAccess *bool
	AdvertiseRoutes        *[]string // nil = unchanged; empty slice = clear all routes
	AcceptRoutes           *bool
	ExitNode               *string // nil = unchanged; "" = clear the exit node
	SSH                    *bool
}

// SetNetworking applies a partial update to networking preferences by
// running a single `tailscale set` invocation with only the flags
// corresponding to non-nil fields in opts.
func (c *Client) SetNetworking(opts NetworkingOptions) error {
	args := buildNetworkingSetArgs(opts)
	if len(args) == 1 {
		// Nothing to change.
		return nil
	}

	cmd := exec.Command(c.binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update networking settings: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// buildNetworkingSetArgs builds the `tailscale set` argv corresponding to
// the non-nil fields in opts. Split out from SetNetworking so the argument
// construction can be unit tested without shelling out.
func buildNetworkingSetArgs(opts NetworkingOptions) []string {
	args := []string{"set"}

	if opts.AdvertiseExitNode != nil {
		args = append(args, fmt.Sprintf("--advertise-exit-node=%t", *opts.AdvertiseExitNode))
	}
	if opts.ExitNodeAllowLANAccess != nil {
		args = append(args, fmt.Sprintf("--exit-node-allow-lan-access=%t", *opts.ExitNodeAllowLANAccess))
	}
	if opts.AdvertiseRoutes != nil {
		args = append(args, "--advertise-routes="+strings.Join(*opts.AdvertiseRoutes, ","))
	}
	if opts.AcceptRoutes != nil {
		args = append(args, fmt.Sprintf("--accept-routes=%t", *opts.AcceptRoutes))
	}
	if opts.ExitNode != nil {
		args = append(args, "--exit-node="+*opts.ExitNode)
	}
	if opts.SSH != nil {
		args = append(args, fmt.Sprintf("--ssh=%t", *opts.SSH))
	}

	return args
}
