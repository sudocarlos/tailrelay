package caddy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/sudocarlos/tailrelay/internal/config"
)

// statefulMock is a minimal in-memory implementation of the Caddy Admin API
// that supports the paths exercised by AddProxy / DeleteProxy for *.ts.net
// proxies. It is not a general-purpose JSON-path engine; it handles only the
// specific GET / PUT / PATCH / POST / DELETE paths that the proxy manager hits.
type statefulMock struct {
	root map[string]interface{}
}

func newStatefulMock() *statefulMock {
	return &statefulMock{root: map[string]interface{}{}}
}

func (m *statefulMock) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip the "/config" prefix that the APIClient prepends.
	p := strings.TrimPrefix(r.URL.Path, "/config")
	if p == "" {
		p = "/"
	}

	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		val := jsonGet(m.root, p)
		if val == nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
			return
		}
		data, _ := json.Marshal(val)
		w.WriteHeader(http.StatusOK)
		w.Write(data)

	case http.MethodPut, http.MethodPatch:
		var body interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		jsonSet(m.root, p, body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))

	case http.MethodPost:
		var body interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		jsonAppend(m.root, p, body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))

	case http.MethodDelete:
		jsonDelete(m.root, p)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// jsonGet returns the value at the JSON path, or nil if not found.
func jsonGet(root map[string]interface{}, path string) interface{} {
	if path == "/" || path == "" {
		return root
	}
	cur := interface{}(root)
	for _, seg := range splitPath(path) {
		switch v := cur.(type) {
		case map[string]interface{}:
			next, ok := v[seg]
			if !ok {
				return nil
			}
			cur = next
		case []interface{}:
			n, err := strconv.Atoi(seg)
			if err != nil || n < 0 || n >= len(v) {
				return nil
			}
			cur = v[n]
		default:
			return nil
		}
	}
	return cur
}

// jsonSet sets the value at path, creating intermediate maps as needed.
func jsonSet(root map[string]interface{}, path string, val interface{}) {
	if path == "/" || path == "" {
		if m, ok := val.(map[string]interface{}); ok {
			for k, v := range m {
				root[k] = v
			}
		}
		return
	}
	segs := splitPath(path)
	cur := root
	for i, seg := range segs {
		if i == len(segs)-1 {
			cur[seg] = val
			return
		}
		next, ok := cur[seg]
		if !ok {
			newMap := map[string]interface{}{}
			cur[seg] = newMap
			cur = newMap
		} else if m, ok := next.(map[string]interface{}); ok {
			cur = m
		} else {
			return
		}
	}
}

// jsonAppend appends val to the array at path. If path points to a non-array,
// it replaces the value.
func jsonAppend(root map[string]interface{}, path string, val interface{}) {
	existing := jsonGet(root, path)
	if arr, ok := existing.([]interface{}); ok {
		jsonSet(root, path, append(arr, val))
	} else {
		jsonSet(root, path, val)
	}
}

// jsonDelete removes the element at path. For numeric final segments it splices
// from the parent array; for string final segments it deletes from the parent map.
func jsonDelete(root map[string]interface{}, path string) {
	if path == "/" || path == "" {
		return
	}
	segs := splitPath(path)
	if len(segs) == 0 {
		return
	}

	lastSeg := segs[len(segs)-1]
	parentPath := "/" + strings.Join(segs[:len(segs)-1], "/")
	parent := jsonGet(root, parentPath)

	switch p := parent.(type) {
	case map[string]interface{}:
		delete(p, lastSeg)
	case []interface{}:
		// The parent itself is an array; this shouldn't occur for our paths
		// but handle gracefully.
		n, err := strconv.Atoi(lastSeg)
		if err == nil && n >= 0 && n < len(p) {
			// We cannot mutate a []interface{} in place by index; we need to
			// update the grandparent. Fall through to the map-key path below.
			_ = n
		}
	default:
		// parent is nil or a scalar — nothing to do
	}

	// Handle the case: parent is a map, last seg is numeric → means the map
	// holds an array under its last map-key sibling. This is the case for
	// DELETE /apps/tls/automation/policies/0 where the parent map is
	// "automation" and policies is an array. We need to look up the array
	// in the grandparent map.
	if _, err := strconv.Atoi(lastSeg); err == nil && len(segs) >= 2 {
		arrayKey := segs[len(segs)-2]
		grandParentPath := "/" + strings.Join(segs[:len(segs)-2], "/")
		var grandParent map[string]interface{}
		if grandParentPath == "/" {
			grandParent = root
		} else {
			gp := jsonGet(root, grandParentPath)
			if gp == nil {
				return
			}
			var ok bool
			grandParent, ok = gp.(map[string]interface{})
			if !ok {
				return
			}
		}
		arr, ok := grandParent[arrayKey].([]interface{})
		if !ok {
			return
		}
		n, _ := strconv.Atoi(lastSeg)
		if n < 0 || n >= len(arr) {
			return
		}
		grandParent[arrayKey] = append(arr[:n], arr[n+1:]...)
	}
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestAddProxy_TailscaleTLS verifies that AddProxy for a *.ts.net hostname
// creates an HTTP server with tls_connection_policies and a TLS automation
// policy with the Tailscale cert manager.
func TestAddProxy_TailscaleTLS(t *testing.T) {
	mock := newStatefulMock()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)

	proxy := config.CaddyProxy{
		ID:       "ts-proxy-1",
		Hostname: "mynode.tailnet.ts.net",
		Port:     21000,
		Target:   "localhost:9000",
		Enabled:  true,
	}

	result, err := pm.AddProxy(proxy)
	if err != nil {
		t.Fatalf("AddProxy returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("AddProxy returned nil result")
	}

	// Verify the HTTP server was created with tls_connection_policies.
	servers, err := pm.listServers()
	if err != nil {
		t.Fatalf("listServers failed: %v", err)
	}
	if len(servers) == 0 {
		t.Fatal("expected at least one server to be created")
	}

	var foundTLSConnPolicy bool
	for _, server := range servers {
		if server == nil {
			continue
		}
		if len(server.TLSConnPolicies) > 0 {
			foundTLSConnPolicy = true
			break
		}
	}
	if !foundTLSConnPolicy {
		t.Error("expected server to have tls_connection_policies for *.ts.net hostname")
	}

	// Verify the TLS automation policy was created with Tailscale cert manager.
	tlsPoliciesRaw := jsonGet(mock.root, "/apps/tls/automation/policies")
	if tlsPoliciesRaw == nil {
		t.Fatal("expected TLS automation policies to be created")
	}

	tlsPoliciesJSON, _ := json.Marshal(tlsPoliciesRaw)
	var policies []struct {
		Subjects       []string `json:"subjects"`
		GetCertificate []struct {
			Via string `json:"via"`
		} `json:"get_certificate"`
		OnDemand bool `json:"on_demand"`
	}
	if err := json.Unmarshal(tlsPoliciesJSON, &policies); err != nil {
		t.Fatalf("failed to parse TLS policies: %v", err)
	}

	var foundTailscalePolicy bool
	for _, p := range policies {
		for _, mgr := range p.GetCertificate {
			if mgr.Via != "tailscale" {
				continue
			}
			foundTailscalePolicy = true
			subjectFound := false
			for _, s := range p.Subjects {
				if strings.EqualFold(NormalizeHostname(s), "mynode.tailnet.ts.net") {
					subjectFound = true
					break
				}
			}
			if !subjectFound {
				t.Errorf("Tailscale TLS policy missing hostname in subjects: %v", p.Subjects)
			}
			if !p.OnDemand {
				t.Error("Tailscale TLS policy should have on_demand: true")
			}
		}
	}
	if !foundTailscalePolicy {
		t.Error("expected a TLS automation policy with Tailscale cert manager")
	}
}

// TestAddProxy_NonTailscale verifies that AddProxy for a non-*.ts.net hostname
// does NOT create tls_connection_policies or a Tailscale TLS policy.
func TestAddProxy_NonTailscale(t *testing.T) {
	mock := newStatefulMock()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)

	proxy := config.CaddyProxy{
		ID:       "plain-proxy",
		Hostname: "internal.example.com",
		Port:     21001,
		Target:   "localhost:9001",
		Enabled:  true,
	}

	if _, err := pm.AddProxy(proxy); err != nil {
		t.Fatalf("AddProxy returned unexpected error: %v", err)
	}

	// Verify no tls_connection_policies on the server.
	servers, err := pm.listServers()
	if err != nil {
		t.Fatalf("listServers failed: %v", err)
	}
	for _, server := range servers {
		if server == nil {
			continue
		}
		if len(server.TLSConnPolicies) > 0 {
			t.Error("non-ts.net proxy should NOT have tls_connection_policies")
		}
	}

	// Verify no Tailscale TLS automation policy was created.
	tlsPoliciesRaw := jsonGet(mock.root, "/apps/tls/automation/policies")
	if tlsPoliciesRaw != nil {
		tlsPoliciesJSON, _ := json.Marshal(tlsPoliciesRaw)
		var policies []struct {
			GetCertificate []struct {
				Via string `json:"via"`
			} `json:"get_certificate"`
		}
		if err := json.Unmarshal(tlsPoliciesJSON, &policies); err == nil {
			for _, p := range policies {
				for _, mgr := range p.GetCertificate {
					if mgr.Via == "tailscale" {
						t.Error("non-ts.net proxy should NOT create a Tailscale TLS policy")
					}
				}
			}
		}
	}
}

// TestUpdateProxyHostnames_RenamesMatchingProxies verifies that
// UpdateProxyHostnames updates the Hostname field of every proxy that matches
// oldFQDN and leaves non-matching proxies untouched.
func TestUpdateProxyHostnames_RenamesMatchingProxies(t *testing.T) {
	mock := newStatefulMock()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)

	const oldFQDN = "oldname.tailnet.ts.net"
	const newFQDN = "newname.tailnet.ts.net"

	// Add two proxies on the old hostname and one on an unrelated hostname.
	proxyA := config.CaddyProxy{
		ID: "proxy-a", Hostname: oldFQDN, Port: 21010, Target: "localhost:9010", Enabled: true,
	}
	proxyB := config.CaddyProxy{
		ID: "proxy-b", Hostname: oldFQDN, Port: 21011, Target: "localhost:9011", Enabled: true,
	}
	proxyC := config.CaddyProxy{
		ID: "proxy-c", Hostname: "other.tailnet.ts.net", Port: 21012, Target: "localhost:9012", Enabled: true,
	}

	for _, p := range []config.CaddyProxy{proxyA, proxyB, proxyC} {
		if _, err := pm.AddProxy(p); err != nil {
			t.Fatalf("AddProxy(%s) failed: %v", p.ID, err)
		}
	}

	// Run the rename.
	if err := pm.UpdateProxyHostnames(oldFQDN, newFQDN); err != nil {
		t.Fatalf("UpdateProxyHostnames returned unexpected error: %v", err)
	}

	// Verify metadata: A and B should now use newFQDN; C unchanged.
	proxies, err := pm.ListProxies()
	if err != nil {
		t.Fatalf("ListProxies failed: %v", err)
	}

	byID := make(map[string]string, len(proxies))
	for _, p := range proxies {
		byID[p.ID] = p.Hostname
	}

	if got := byID["proxy-a"]; !strings.EqualFold(NormalizeHostname(got), NormalizeHostname(newFQDN)) {
		t.Errorf("proxy-a: expected hostname %q, got %q", newFQDN, got)
	}
	if got := byID["proxy-b"]; !strings.EqualFold(NormalizeHostname(got), NormalizeHostname(newFQDN)) {
		t.Errorf("proxy-b: expected hostname %q, got %q", newFQDN, got)
	}
	if got := byID["proxy-c"]; strings.EqualFold(NormalizeHostname(got), NormalizeHostname(newFQDN)) {
		t.Errorf("proxy-c should NOT have been renamed, but got hostname %q", got)
	}
}

// TestUpdateProxyHostnames_NoOp verifies that passing equal old and new FQDNs
// returns no error and does not modify any proxy.
func TestUpdateProxyHostnames_NoOp(t *testing.T) {
	mock := newStatefulMock()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)

	const fqdn = "same.tailnet.ts.net"
	proxy := config.CaddyProxy{
		ID: "proxy-noop", Hostname: fqdn, Port: 21020, Target: "localhost:9020", Enabled: true,
	}
	if _, err := pm.AddProxy(proxy); err != nil {
		t.Fatalf("AddProxy failed: %v", err)
	}

	if err := pm.UpdateProxyHostnames(fqdn, fqdn); err != nil {
		t.Fatalf("UpdateProxyHostnames(same,same) returned unexpected error: %v", err)
	}

	// Hostname should be unchanged.
	proxies, err := pm.ListProxies()
	if err != nil {
		t.Fatalf("ListProxies failed: %v", err)
	}
	for _, p := range proxies {
		if p.ID == proxy.ID && !strings.EqualFold(NormalizeHostname(p.Hostname), NormalizeHostname(fqdn)) {
			t.Errorf("hostname changed unexpectedly: got %q", p.Hostname)
		}
	}
}

// *.ts.net hostname removes the TLS automation policy.
func TestDeleteProxy_CleansTLSPolicy(t *testing.T) {
	mock := newStatefulMock()
	srv := httptest.NewServer(mock)
	defer srv.Close()

	pm := newTestProxyManager(t, srv.URL)

	proxy := config.CaddyProxy{
		ID:       "ts-proxy-delete",
		Hostname: "delete-me.tailnet.ts.net",
		Port:     21002,
		Target:   "localhost:9002",
		Enabled:  true,
	}

	if _, err := pm.AddProxy(proxy); err != nil {
		t.Fatalf("AddProxy failed: %v", err)
	}

	// Confirm TLS policy was created.
	if jsonGet(mock.root, "/apps/tls/automation/policies") == nil {
		t.Fatal("TLS policies should exist after AddProxy")
	}

	// Delete the proxy.
	if err := pm.DeleteProxy(proxy.ID); err != nil {
		t.Fatalf("DeleteProxy failed: %v", err)
	}

	// The TLS policies array should now be empty.
	tlsPoliciesRaw := jsonGet(mock.root, "/apps/tls/automation/policies")
	if tlsPoliciesRaw != nil {
		tlsPoliciesJSON, _ := json.Marshal(tlsPoliciesRaw)
		var policies []interface{}
		if err := json.Unmarshal(tlsPoliciesJSON, &policies); err == nil && len(policies) > 0 {
			t.Errorf("expected TLS policies to be empty after deleting last proxy, got: %s", tlsPoliciesJSON)
		}
	}
}
