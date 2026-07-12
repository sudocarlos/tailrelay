package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"strings"

	"github.com/sudocarlos/tailrelay/internal/tailscale"
)

// APINetworking returns the current networking preferences (exit node
// advertisement, accepted/advertised routes, SSH) as JSON.
func (h *TailscaleHandler) APINetworking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary, err := h.tsClient.GetNetworkingSummary()
	if err != nil {
		log.Printf("Error getting Tailscale networking preferences: %v", err)
		writeJSONError(w, "Failed to get networking preferences", http.StatusInternalServerError)
		return
	}

	writeJSON(w, summary)
}

// networkingUpdateRequest is a partial update to networking preferences.
// Fields are pointers so that "absent from the request" (leave unchanged)
// can be distinguished from an explicit zero value (false, "", or []).
type networkingUpdateRequest struct {
	AdvertiseExitNode      *bool     `json:"advertise_exit_node"`
	ExitNodeAllowLANAccess *bool     `json:"exit_node_allow_lan_access"`
	AdvertiseRoutes        *[]string `json:"advertise_routes"`
	AcceptRoutes           *bool     `json:"accept_routes"`
	ExitNode               *string   `json:"exit_node"`
	SSH                    *bool     `json:"ssh"`
}

// UpdateNetworking applies a partial update to networking preferences via
// `tailscale set`. Only the fields present in the request body are changed;
// omitted fields are left untouched.
func (h *TailscaleHandler) UpdateNetworking(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body networkingUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSONError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.AdvertiseRoutes != nil {
		if err := validateAdvertiseRoutes(*body.AdvertiseRoutes); err != nil {
			writeJSONError(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	opts := tailscale.NetworkingOptions{
		AdvertiseExitNode:      body.AdvertiseExitNode,
		ExitNodeAllowLANAccess: body.ExitNodeAllowLANAccess,
		AdvertiseRoutes:        body.AdvertiseRoutes,
		AcceptRoutes:           body.AcceptRoutes,
		ExitNode:               body.ExitNode,
		SSH:                    body.SSH,
	}

	if err := h.tsClient.SetNetworking(opts); err != nil {
		log.Printf("Error updating Tailscale networking preferences: %v", err)
		writeJSONError(w, "Failed to update networking settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{
		"status":  "success",
		"message": "Networking settings updated",
	})
}

// validateAdvertiseRoutes ensures each route is a valid CIDR with no host
// bits set, and rejects the reserved exit-node CIDRs (0.0.0.0/0, ::/0) —
// those are controlled exclusively via the "advertise as exit node" toggle,
// since that's how Tailscale implements exit-node advertisement internally.
func validateAdvertiseRoutes(routes []string) error {
	for _, route := range routes {
		trimmed := strings.TrimSpace(route)
		prefix, err := netip.ParsePrefix(trimmed)
		if err != nil {
			return fmt.Errorf("invalid route %q: must be a CIDR, e.g. 192.168.1.0/24", route)
		}
		if prefix.Bits() == 0 {
			return fmt.Errorf(`route %q is not allowed here: use the "advertise as exit node" toggle instead`, route)
		}
		if prefix.Masked() != prefix {
			return fmt.Errorf("invalid route %q: host bits must be zero, e.g. use %s", route, prefix.Masked())
		}
	}
	return nil
}
