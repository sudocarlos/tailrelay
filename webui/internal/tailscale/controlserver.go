package tailscale

import (
	"fmt"
	"net/url"
)

// ValidateControlServerURL checks that raw is a valid custom control server
// URL suitable for `tailscale login --login-server=<url>` (e.g. a
// self-hosted Headscale instance). An empty string is always valid — it
// means "use Tailscale's default control plane".
func ValidateControlServerURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("invalid control server URL: must be an http(s) URL, e.g. https://headscale.example.com")
	}
	return nil
}

// buildLoginWithAuthKeyArgs builds the `tailscale up` argv used by
// LoginWithAuthKey, appending --login-server only when controlServer is
// set. Split out from LoginWithAuthKey so the argument construction can be
// unit tested without shelling out, mirroring buildNetworkingSetArgs.
func buildLoginWithAuthKeyArgs(key, controlServer string) []string {
	args := []string{"up", "--authkey=" + key}
	if controlServer != "" {
		args = append(args, "--login-server="+controlServer)
	}
	return args
}
