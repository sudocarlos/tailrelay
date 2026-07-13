package tailscale

import (
	"reflect"
	"testing"
)

func TestSummarizeNetworking(t *testing.T) {
	tests := []struct {
		name  string
		prefs *Prefs
		want  *NetworkingSummary
	}{
		{
			name:  "no advertised routes",
			prefs: &Prefs{},
			want:  &NetworkingSummary{},
		},
		{
			name: "custom subnet routes only",
			prefs: &Prefs{
				AdvertiseRoutes: []string{"10.0.0.0/8", "192.168.1.0/24"},
			},
			want: &NetworkingSummary{
				AdvertiseRoutes: []string{"10.0.0.0/8", "192.168.1.0/24"},
			},
		},
		{
			name: "exit node advertised alongside custom routes",
			prefs: &Prefs{
				AdvertiseRoutes: []string{"10.0.0.0/8", "0.0.0.0/0", "::/0"},
			},
			want: &NetworkingSummary{
				AdvertiseExitNode: true,
				AdvertiseRoutes:   []string{"10.0.0.0/8"},
			},
		},
		{
			name: "only one of the exit-node CIDRs present is not enough to mark advertised, but is still excluded from custom routes",
			prefs: &Prefs{
				AdvertiseRoutes: []string{"0.0.0.0/0"},
			},
			want: &NetworkingSummary{},
		},
		{
			name: "full prefs mapped through",
			prefs: &Prefs{
				RouteAll:               true,
				ExitNodeIP:             "100.64.0.1",
				ExitNodeAllowLANAccess: true,
				RunSSH:                 true,
				AdvertiseRoutes:        []string{"0.0.0.0/0", "::/0"},
			},
			want: &NetworkingSummary{
				AdvertiseExitNode:      true,
				ExitNodeAllowLANAccess: true,
				AcceptRoutes:           true,
				ExitNode:               "100.64.0.1",
				SSH:                    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeNetworking(tt.prefs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("summarizeNetworking() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestBuildNetworkingSetArgs(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	strPtr := func(s string) *string { return &s }
	routesPtr := func(r []string) *[]string { return &r }

	tests := []struct {
		name string
		opts NetworkingOptions
		want []string
	}{
		{
			name: "no fields set produces bare set with nothing to change",
			opts: NetworkingOptions{},
			want: []string{"set"},
		},
		{
			name: "single toggle",
			opts: NetworkingOptions{AdvertiseExitNode: boolPtr(true)},
			want: []string{"set", "--advertise-exit-node=true"},
		},
		{
			name: "clearing exit node uses empty flag value",
			opts: NetworkingOptions{ExitNode: strPtr("")},
			want: []string{"set", "--exit-node="},
		},
		{
			name: "clearing routes uses empty flag value",
			opts: NetworkingOptions{AdvertiseRoutes: routesPtr([]string{})},
			want: []string{"set", "--advertise-routes="},
		},
		{
			name: "multiple routes joined by comma",
			opts: NetworkingOptions{AdvertiseRoutes: routesPtr([]string{"10.0.0.0/8", "192.168.1.0/24"})},
			want: []string{"set", "--advertise-routes=10.0.0.0/8,192.168.1.0/24"},
		},
		{
			name: "all fields combined into one invocation",
			opts: NetworkingOptions{
				AdvertiseExitNode:      boolPtr(true),
				ExitNodeAllowLANAccess: boolPtr(false),
				AdvertiseRoutes:        routesPtr([]string{"10.0.0.0/8"}),
				AcceptRoutes:           boolPtr(true),
				ExitNode:               strPtr("100.64.0.1"),
				SSH:                    boolPtr(true),
			},
			want: []string{
				"set",
				"--advertise-exit-node=true",
				"--exit-node-allow-lan-access=false",
				"--advertise-routes=10.0.0.0/8",
				"--accept-routes=true",
				"--exit-node=100.64.0.1",
				"--ssh=true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildNetworkingSetArgs(tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildNetworkingSetArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
