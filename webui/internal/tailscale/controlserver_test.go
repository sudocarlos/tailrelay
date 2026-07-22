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
	got := buildLoginArgs("https://headscale.example.com")
	want := []string{"login", "--login-server=https://headscale.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildLoginArgs(...) = %v, want %v", got, want)
	}
}

func TestBuildLoginWithAuthKeyArgs(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		controlServer string
		want          []string
	}{
		{
			name: "no control server",
			key:  "tskey-auth-abc123",
			want: []string{"up", "--authkey=tskey-auth-abc123"},
		},
		{
			name:          "with custom control server",
			key:           "tskey-auth-abc123",
			controlServer: "https://headscale.example.com",
			want:          []string{"up", "--authkey=tskey-auth-abc123", "--login-server=https://headscale.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLoginWithAuthKeyArgs(tt.key, tt.controlServer)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildLoginWithAuthKeyArgs(%q, %q) = %v, want %v", tt.key, tt.controlServer, got, tt.want)
			}
		})
	}
}

func TestBuildUpWithHostnameArgs(t *testing.T) {
	tests := []struct {
		name          string
		hostname      string
		controlServer string
		want          []string
	}{
		{
			name:     "no control server",
			hostname: "my-device",
			want:     []string{"up", "--hostname=my-device", "--reset"},
		},
		{
			name:          "with custom control server",
			hostname:      "my-device",
			controlServer: "https://headscale.example.com",
			want:          []string{"up", "--hostname=my-device", "--reset", "--login-server=https://headscale.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildUpWithHostnameArgs(tt.hostname, tt.controlServer)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildUpWithHostnameArgs(%q, %q) = %v, want %v", tt.hostname, tt.controlServer, got, tt.want)
			}
		})
	}
}
