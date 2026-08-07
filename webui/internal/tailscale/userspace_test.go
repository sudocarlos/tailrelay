package tailscale

import "testing"

func TestCmdlineHasUserspaceNetworking(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "empty argv",
			args: nil,
			want: false,
		},
		{
			name: "plain tailscaled no tun flag",
			args: []string{"/usr/sbin/tailscaled", "--socket=/var/run/tailscale/tailscaled.sock"},
			want: false,
		},
		{
			name: "real tun device",
			args: []string{"tailscaled", "--tun=ts0"},
			want: false,
		},
		{
			name: "userspace networking flag present",
			args: []string{"tailscaled", "--state=/var/lib/tailscale/tailscaled.state", "--tun=userspace-networking", "--socks5-server=localhost:1055"},
			want: true,
		},
		{
			name: "flag must match exactly, not as substring",
			args: []string{"tailscaled", "--tun=userspace-networking-something"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cmdlineHasUserspaceNetworking(tt.args); got != tt.want {
				t.Errorf("cmdlineHasUserspaceNetworking(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestClientDetectUserspaceNetworking_Override(t *testing.T) {
	c := NewClient()
	if c.DetectUserspaceNetworking() {
		t.Fatalf("expected default detection to report false when no tailscaled is running in the test environment")
	}

	c.SetUserspaceNetworkingDetectorForTest(func() bool { return true })
	if !c.DetectUserspaceNetworking() {
		t.Fatalf("expected injected detector to report true")
	}

	c.SetUserspaceNetworkingDetectorForTest(func() bool { return false })
	if c.DetectUserspaceNetworking() {
		t.Fatalf("expected injected detector to report false")
	}
}