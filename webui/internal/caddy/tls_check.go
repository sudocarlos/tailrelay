package caddy

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/sudocarlos/tailrelay/internal/config"
	"github.com/sudocarlos/tailrelay/internal/logger"
)

const (
	// tlsProbeTimeout is the maximum time to wait for a TLS handshake when
	// probing each proxy's certificate. Kept short so the API stays responsive.
	tlsProbeTimeout = 3 * time.Second
)

// probeTLSCert attempts a TLS handshake to localhost:<port> using the given
// hostname as the SNI / ServerName. It does NOT verify the certificate chain
// against system roots because the cert may be from Tailscale's own CA.
// Instead it checks that the server presents a certificate whose Subject
// or SAN matches hostname and that is not expired.
//
// A non-empty returned string is a human-readable error message suitable for
// display in the Web UI.
func probeTLSCert(hostname string, port int) string {
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	conn, err := net.DialTimeout("tcp", addr, tlsProbeTimeout)
	if err != nil {
		return fmt.Sprintf("cannot reach proxy on port %d: %s", port, err.Error())
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(tlsProbeTimeout)); err != nil {
		return fmt.Sprintf("failed to set connection deadline: %s", err.Error())
	}

	tlsConn := tls.Client(conn, &tls.Config{
		ServerName:         NormalizeHostname(hostname),
		InsecureSkipVerify: true, //nolint:gosec // we do our own validation below
	})

	if err := tlsConn.Handshake(); err != nil {
		return fmt.Sprintf("TLS handshake failed (certificate may not be provisioned yet): %s", err.Error())
	}
	defer tlsConn.Close()

	// Inspect the leaf certificate to catch expired/wrong-host issues.
	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "server presented no TLS certificate"
	}

	leaf := certs[0]
	now := time.Now()

	if now.After(leaf.NotAfter) {
		return fmt.Sprintf("TLS certificate expired on %s", leaf.NotAfter.UTC().Format("2006-01-02"))
	}

	if now.Before(leaf.NotBefore) {
		return fmt.Sprintf("TLS certificate not yet valid until %s", leaf.NotBefore.UTC().Format("2006-01-02"))
	}

	target := NormalizeHostname(hostname)
	if err := leaf.VerifyHostname(target); err != nil {
		return fmt.Sprintf("TLS certificate does not cover hostname %q: %s", target, err.Error())
	}

	return "" // all good
}

// CheckProxyCerts probes TLS certificates for all enabled proxies whose
// hostname is a MagicDNS *.ts.net address. Probes run concurrently and the
// results are returned as a map of proxyID → error string (empty = OK).
//
// Disabled proxies are skipped because Caddy does not serve them, so there
// is no certificate to check.
func CheckProxyCerts(proxies []config.CaddyProxy) map[string]string {
	type job struct {
		proxy config.CaddyProxy
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make(map[string]string)
	)

	for _, p := range proxies {
		if !p.Enabled {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(NormalizeHostname(p.Hostname)), ".ts.net") {
			continue
		}

		wg.Add(1)
		go func(proxy config.CaddyProxy) {
			defer wg.Done()

			logger.Debug("caddy", "Probing TLS cert for proxy %s (%s:%d)", proxy.ID, proxy.Hostname, proxy.Port)
			errMsg := probeTLSCert(proxy.Hostname, proxy.Port)

			mu.Lock()
			results[proxy.ID] = errMsg
			mu.Unlock()

			if errMsg != "" {
				logger.Warn("caddy", "TLS cert issue for proxy %s (%s): %s", proxy.ID, proxy.Hostname, errMsg)
			}
		}(p)
	}

	wg.Wait()
	return results
}
