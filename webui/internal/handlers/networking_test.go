package handlers

import "testing"

func TestValidateAdvertiseRoutes(t *testing.T) {
	tests := []struct {
		name    string
		routes  []string
		wantErr bool
	}{
		{name: "empty list is valid", routes: []string{}, wantErr: false},
		{name: "single valid IPv4 CIDR", routes: []string{"192.168.1.0/24"}, wantErr: false},
		{name: "multiple valid CIDRs", routes: []string{"10.0.0.0/8", "192.168.1.0/24"}, wantErr: false},
		{name: "valid IPv6 CIDR", routes: []string{"fd7a:115c:a1e0::/48"}, wantErr: false},
		{name: "not a CIDR at all", routes: []string{"not-an-ip"}, wantErr: true},
		{name: "bare IP without prefix length", routes: []string{"192.168.1.5"}, wantErr: true},
		{name: "host bits set", routes: []string{"192.168.1.5/24"}, wantErr: true},
		{name: "rejects exit-node IPv4 default route", routes: []string{"0.0.0.0/0"}, wantErr: true},
		{name: "rejects exit-node IPv6 default route", routes: []string{"::/0"}, wantErr: true},
		{name: "one bad entry among good ones still fails", routes: []string{"10.0.0.0/8", "bad"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAdvertiseRoutes(tt.routes)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAdvertiseRoutes(%v) error = %v, wantErr %v", tt.routes, err, tt.wantErr)
			}
		})
	}
}
