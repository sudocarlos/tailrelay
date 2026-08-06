package tailscale

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// userspaceTunFlag is the tailscaled flag that enables userspace-networking
// mode. In this mode the daemon has no kernel TUN device and cannot install
// host routes, so it cannot redirect the *host's own* outbound traffic
// through a peer exit node — only inbound tailnet traffic (i.e. *running as*
// an exit node for others) works. tailrelay always starts tailscaled with
// this flag (see start.sh) to avoid requiring NET_ADMIN or /dev/net/tun.
const userspaceTunFlag = "--tun=userspace-networking"

// DetectUserspaceNetworking reports whether tailscaled was started with
// --tun=userspace-networking. The Web UI uses this to hide/forbid selecting
// a peer as an exit node (a silent no-op under userspace networking) while
// still allowing "Run as exit node".
func (c *Client) DetectUserspaceNetworking() bool {
	if c.detectUserspace != nil {
		return c.detectUserspace()
	}
	return defaultDetectUserspaceNetworking()
}

// SetUserspaceNetworkingDetectorForTest overrides the userspace-networking
// detection used by this client. Intended only for tests that need to force
// a mode without a running tailscaled.
func (c *Client) SetUserspaceNetworkingDetectorForTest(fn func() bool) {
	c.detectUserspace = fn
}

// defaultDetectUserspaceNetworking reports the daemon's networking mode by
// scanning /proc on Linux; on any other platform (or when /proc or a
// tailscaled process is unavailable) it reports false, letting the UI fall
// back to exposing the full exit-node dropdown.
func defaultDetectUserspaceNetworking() bool {
	args, ok := readTailscaledCmdline()
	if !ok {
		return false
	}
	return cmdlineHasUserspaceNetworking(args)
}

// cmdlineHasUserspaceNetworking reports whether argv contains the
// userspace-networking flag. Split out from defaultDetectUserspaceNetworking
// so the parsing can be unit tested without a running tailscaled.
func cmdlineHasUserspaceNetworking(args []string) bool {
	for _, a := range args {
		if a == userspaceTunFlag {
			return true
		}
	}
	return false
}

// readTailscaledCmdline returns the argv of the running tailscaled process
// found under /proc, or (nil, false) if it cannot be determined. Only
// meaningful on Linux; on other platforms it returns false immediately.
func readTailscaledCmdline() ([]string, bool) {
	if runtime.GOOS != "linux" {
		return nil, false
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue // not a PID directory
		}
		data, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		// /proc/<pid>/cmdline is NUL-separated, with a trailing NUL.
		args := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if len(args) == 0 || args[0] == "" {
			continue
		}
		if !strings.Contains(filepath.Base(args[0]), "tailscaled") {
			continue
		}
		return args, true
	}
	return nil, false
}