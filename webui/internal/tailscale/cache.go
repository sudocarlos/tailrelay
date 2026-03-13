package tailscale

import (
	"context"
	"sync"
	"time"
)

const statusCachePollInterval = 15 * time.Second

// StatusCache holds a periodically-refreshed record of whether Tailscale is
// in the "Running" state. It is safe for concurrent use.
//
// The cache starts in the not-ready state and transitions to ready as soon as
// the background poller observes BackendState == "Running". This means callers
// that depend on Tailscale being fully connected (e.g. TLS cert probes) will
// not fire until the daemon has actually authenticated and joined the tailnet.
type StatusCache struct {
	mu     sync.RWMutex
	ready  bool
	client *Client
}

// NewStatusCache returns a StatusCache backed by the given client. The cache
// is initialised to not-ready; call Start to begin background polling.
func NewStatusCache(client *Client) *StatusCache {
	return &StatusCache{client: client}
}

// Start launches a background goroutine that polls IsConnected every
// statusCachePollInterval and updates the cached state. It returns
// immediately; the goroutine exits when ctx is cancelled.
func (c *StatusCache) Start(ctx context.Context) {
	go func() {
		// Poll immediately on startup so the cache warms up quickly.
		c.poll()

		ticker := time.NewTicker(statusCachePollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.poll()
			}
		}
	}()
}

// IsReady reports whether the last poll found Tailscale in the Running state.
// It never blocks and never performs I/O.
func (c *StatusCache) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

// poll calls IsConnected and updates the cached state.
func (c *StatusCache) poll() {
	connected, err := c.client.IsConnected()
	if err != nil {
		// On error leave the previous state unchanged — a transient socket
		// hiccup should not flip a running tailscale to "not ready".
		return
	}

	c.mu.Lock()
	c.ready = connected
	c.mu.Unlock()
}
