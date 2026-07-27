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

// buildLoginArgs builds the `tailscale login` argv used by Login's
// CLI-based flow when a custom control server and/or a persisted hostname
// preference is configured. hostname reapplies the last name set via
// ChangeHostname so a Logout followed by re-authenticating doesn't silently
// fall back to tailscaled's OS default hostname. Split out so the argument
// construction can be unit tested without shelling out, mirroring
// buildLoginWithAuthKeyArgs.
func buildLoginArgs(controlServer, hostname string) []string {
	args := []string{"login"}
	if controlServer != "" {
		args = append(args, "--login-server="+controlServer)
	}
	if hostname != "" {
		args = append(args, "--hostname="+hostname)
	}
	return args
}

// buildLoginWithAuthKeyArgs builds the `tailscale up` argv used by
// LoginWithAuthKey, appending --login-server and --hostname only when set.
// hostname reapplies the last name set via ChangeHostname so a Logout
// followed by re-authenticating doesn't silently fall back to tailscaled's
// OS default hostname. Split out from LoginWithAuthKey so the argument
// construction can be unit tested without shelling out, mirroring
// buildNetworkingSetArgs.
func buildLoginWithAuthKeyArgs(key, controlServer, hostname string) []string {
	args := []string{"up", "--authkey=" + key}
	if controlServer != "" {
		args = append(args, "--login-server="+controlServer)
	}
	if hostname != "" {
		args = append(args, "--hostname="+hostname)
	}
	return args
}

// buildSetHostnameArgs builds the selective `tailscale set` invocation used
// to rename the device without resetting unrelated preferences.
func buildSetHostnameArgs(hostname string) []string {
	return []string{"set", "--hostname=" + hostname}
}
