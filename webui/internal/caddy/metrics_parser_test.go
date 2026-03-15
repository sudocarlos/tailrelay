package caddy

import (
	"testing"
)

// samplePrometheus is representative output from Caddy's /metrics endpoint
// for two proxies that share the same Tailscale FQDN but run on different
// Caddy servers (srv0 → :8080, srv1 → :9090).
const samplePrometheus = `
# HELP caddy_http_requests_total Counter of HTTP(S) requests made.
# TYPE caddy_http_requests_total counter
caddy_http_requests_total{code="200",handler="subroute",host="mynode.ts.net",method="GET",server="srv0"} 10
caddy_http_requests_total{code="200",handler="subroute",host="mynode.ts.net",method="GET",server="srv1"} 3
caddy_http_requests_total{code="200",handler="static_response",host="mynode.ts.net",method="GET",server="srv0"} 99

# HELP caddy_http_request_size_bytes_sum Total size of requests in bytes.
# TYPE caddy_http_request_size_bytes_sum counter
caddy_http_request_size_bytes_sum{handler="subroute",host="mynode.ts.net",server="srv0"} 1024
caddy_http_request_size_bytes_sum{handler="subroute",host="mynode.ts.net",server="srv1"} 256

# HELP caddy_http_response_size_bytes_sum Total size of responses in bytes.
# TYPE caddy_http_response_size_bytes_sum counter
caddy_http_response_size_bytes_sum{handler="subroute",host="mynode.ts.net",server="srv0"} 8192
caddy_http_response_size_bytes_sum{handler="subroute",host="mynode.ts.net",server="srv1"} 512

# HELP caddy_http_request_duration_seconds_count Count of HTTP request durations.
# TYPE caddy_http_request_duration_seconds_count counter
caddy_http_request_duration_seconds_count{code="200",handler="subroute",host="mynode.ts.net",server="srv0"} 10
caddy_http_request_duration_seconds_count{code="404",handler="subroute",host="mynode.ts.net",server="srv1"} 3

# HELP caddy_reverse_proxy_upstreams_healthy Health of reverse proxy upstreams.
# TYPE caddy_reverse_proxy_upstreams_healthy gauge
caddy_reverse_proxy_upstreams_healthy{upstream="backend-a:8080"} 1
caddy_reverse_proxy_upstreams_healthy{upstream="backend-b:9090"} 0
`

// sampleNoServerLabel simulates older Caddy output without the "server" label.
const sampleNoServerLabel = `
caddy_http_requests_total{code="200",handler="reverse_proxy",host="other.ts.net",method="GET"} 5
caddy_http_request_size_bytes_sum{handler="reverse_proxy",host="other.ts.net"} 100
caddy_http_response_size_bytes_sum{handler="reverse_proxy",host="other.ts.net"} 200
`

func TestParseMetrics_ServerLabel(t *testing.T) {
	data := ParseMetrics([]byte(samplePrometheus))

	if len(data.Hosts) != 2 {
		t.Fatalf("expected 2 host entries (one per server), got %d", len(data.Hosts))
	}

	byServer := map[string]HostMetrics{}
	for _, hm := range data.Hosts {
		byServer[hm.Server] = hm
	}

	// Both servers carry the same host FQDN.
	for _, srv := range []string{"srv0", "srv1"} {
		hm, ok := byServer[srv]
		if !ok {
			t.Errorf("missing entry for server %q", srv)
			continue
		}
		if hm.Host != "mynode.ts.net" {
			t.Errorf("%s: expected host mynode.ts.net, got %q", srv, hm.Host)
		}
	}

	srv0 := byServer["srv0"]
	if srv0.Requests != 10 {
		t.Errorf("srv0: expected 10 requests, got %v", srv0.Requests)
	}
	if srv0.RequestsIn != 1024 {
		t.Errorf("srv0: expected 1024 bytes in, got %v", srv0.RequestsIn)
	}
	if srv0.ResponsesOut != 8192 {
		t.Errorf("srv0: expected 8192 bytes out, got %v", srv0.ResponsesOut)
	}
	if srv0.StatusCodes["2xx"] != 10 {
		t.Errorf("srv0: expected 10 2xx, got %v", srv0.StatusCodes["2xx"])
	}

	// static_response handler must NOT be counted (not reverse_proxy/subroute).
	if srv0.Requests == 10+99 {
		t.Errorf("srv0: static_response handler was incorrectly counted")
	}

	srv1 := byServer["srv1"]
	if srv1.Requests != 3 {
		t.Errorf("srv1: expected 3 requests, got %v", srv1.Requests)
	}
	if srv1.StatusCodes["4xx"] != 3 {
		t.Errorf("srv1: expected 3 4xx, got %v", srv1.StatusCodes["4xx"])
	}

	if len(data.Upstreams) != 2 {
		t.Fatalf("expected 2 upstreams, got %d", len(data.Upstreams))
	}
}

func TestParseMetrics_NoServerLabel_FallsBackToHost(t *testing.T) {
	data := ParseMetrics([]byte(sampleNoServerLabel))

	if len(data.Hosts) != 1 {
		t.Fatalf("expected 1 host entry, got %d", len(data.Hosts))
	}
	hm := data.Hosts[0]
	// When there is no server label the key is the host value itself.
	if hm.Server != "other.ts.net" {
		t.Errorf("expected Server=other.ts.net (fallback key), got %q", hm.Server)
	}
	if hm.Host != "other.ts.net" {
		t.Errorf("expected Host=other.ts.net, got %q", hm.Host)
	}
	if hm.Requests != 5 {
		t.Errorf("expected 5 requests, got %v", hm.Requests)
	}
}
