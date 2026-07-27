package tailscale

import (
	"reflect"
	"testing"
)

func TestValidateControlServerURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "empty is valid (default control plane)", raw: "", wantErr: false},
		{name: "valid https URL", raw: "https://headscale.example.com", wantErr: false},
		{name: "valid http URL", raw: "http://headscale.internal:8080", wantErr: false},
		{name: "missing scheme", raw: "headscale.example.com", wantErr: true},
		{name: "unsupported scheme", raw: "ftp://headscale.example.com", wantErr: true},
		{name: "scheme with no host", raw: "https://", wantErr: true},
		{name: "not a URL at all", raw: "not a url", wantErr: true},
		{name: "with query string", raw: "https://headscale.example.com?foo=bar", wantErr: false},
		{name: "with userinfo", raw: "https://user:pass@headscale.example.com", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateControlServerURL(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateControlServerURL(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
		})
	}
}

func TestBuildLoginArgs(t *testing.T) {
	tests := []struct {
		name          string
		controlServer string
		hostname      string
		want          []string
	}{
		{
			name:          "control server only",
			controlServer: "https://headscale.example.com",
			want:          []string{"login", "--login-server=https://headscale.example.com"},
		},
		{
			name:     "hostname only",
			hostname: "my-device",
			want:     []string{"login", "--hostname=my-device"},
		},
		{
			name:          "control server and hostname",
			controlServer: "https://headscale.example.com",
			hostname:      "my-device",
			want:          []string{"login", "--login-server=https://headscale.example.com", "--hostname=my-device"},
		},
		{
			name: "neither set",
			want: []string{"login"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLoginArgs(tt.controlServer, tt.hostname)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildLoginArgs(%q, %q) = %v, want %v", tt.controlServer, tt.hostname, got, tt.want)
			}
		})
	}
}

func TestBuildLoginWithAuthKeyArgs(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		controlServer string
		hostname      string
		want          []string
	}{
		{
			name: "no control server or hostname",
			key:  "tskey-auth-abc123",
			want: []string{"up", "--authkey=tskey-auth-abc123"},
		},
		{
			name:          "with custom control server",
			key:           "tskey-auth-abc123",
			controlServer: "https://headscale.example.com",
			want:          []string{"up", "--authkey=tskey-auth-abc123", "--login-server=https://headscale.example.com"},
		},
		{
			name:     "with hostname",
			key:      "tskey-auth-abc123",
			hostname: "my-device",
			want:     []string{"up", "--authkey=tskey-auth-abc123", "--hostname=my-device"},
		},
		{
			name:          "with custom control server and hostname",
			key:           "tskey-auth-abc123",
			controlServer: "https://headscale.example.com",
			hostname:      "my-device",
			want:          []string{"up", "--authkey=tskey-auth-abc123", "--login-server=https://headscale.example.com", "--hostname=my-device"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLoginWithAuthKeyArgs(tt.key, tt.controlServer, tt.hostname)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildLoginWithAuthKeyArgs(%q, %q, %q) = %v, want %v", tt.key, tt.controlServer, tt.hostname, got, tt.want)
			}
		})
	}
}

func TestBuildSetHostnameArgs(t *testing.T) {
	want := []string{"set", "--hostname=my-device"}
	if got := buildSetHostnameArgs("my-device"); !reflect.DeepEqual(got, want) {
		t.Errorf("buildSetHostnameArgs() = %v, want %v", got, want)
	}
}
