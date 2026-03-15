package caddy

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

// maxRetention is the maximum age of snapshots to keep.
const maxRetention = 31 * 24 * time.Hour

// snapshotInterval is how often the background poller samples Caddy metrics.
const snapshotInterval = 15 * time.Second

// flushInterval is how often accumulated snapshots are written to disk.
const flushInterval = 5 * time.Minute

// MetricsSnapshot is one timestamped sample of cumulative host counters.
// Because Caddy exposes monotonically-increasing counters, a single snapshot
// is not directly useful; the MetricsStore computes deltas between two
// snapshots to answer window queries.
type MetricsSnapshot struct {
	At    time.Time              `json:"at"`
	Hosts map[string]HostMetrics `json:"hosts"` // keyed by host label
}

// metricsHistory is the on-disk/wire format for the full snapshot list.
type metricsHistory struct {
	Snapshots []MetricsSnapshot `json:"snapshots"`
}

// MetricsStore accumulates per-host counter snapshots sampled from Caddy and
// answers delta queries over configurable time windows.  It is safe for
// concurrent use.
//
// On-disk persistence: the store periodically writes all snapshots to a JSON
// file so that history survives container restarts.  Snapshots older than
// maxRetention are pruned automatically.
type MetricsStore struct {
	mu        sync.RWMutex
	snapshots []MetricsSnapshot
	storePath string
}

// NewMetricsStore creates an empty MetricsStore that persists to storePath.
// If storePath is non-empty it attempts to load any existing history from disk.
func NewMetricsStore(storePath string) *MetricsStore {
	ms := &MetricsStore{storePath: storePath}
	if storePath != "" {
		if err := ms.load(); err != nil && !os.IsNotExist(err) {
			log.Printf("metrics_store: failed to load history from %s: %v", storePath, err)
		}
	}
	return ms
}

// AddSnapshot appends a new timestamped snapshot, pruning entries older than
// maxRetention, then returns immediately without blocking callers.
func (ms *MetricsStore) AddSnapshot(snap MetricsSnapshot) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.snapshots = append(ms.snapshots, snap)
	ms.prune()
}

// Query computes the delta of all host counters over the given window duration.
// window == 0 returns the absolute latest snapshot values (current totals).
// For each host the returned value is:
//
//	newest_snapshot_value - oldest_snapshot_value_within_window
//
// This correctly handles paused relays: because we keep all historical
// snapshots, a relay that was removed from Caddy mid-window still contributes
// the traffic it accumulated before being paused.
func (ms *MetricsStore) Query(window time.Duration) *MetricsData {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if len(ms.snapshots) == 0 {
		return &MetricsData{}
	}

	newest := ms.snapshots[len(ms.snapshots)-1]

	if window == 0 {
		// No window requested — return the latest snapshot values directly.
		return snapshotToMetricsData(newest)
	}

	cutoff := newest.At.Add(-window)
	// Find the oldest snapshot within the window.
	oldest := ms.snapshots[0]
	for _, s := range ms.snapshots {
		if !s.At.Before(cutoff) {
			oldest = s
			break
		}
	}

	// Delta: for every host in the newest snapshot, subtract the counter value
	// from the oldest snapshot in the window (if it existed then).
	result := &MetricsData{}
	for host, newH := range newest.Hosts {
		delta := HostMetrics{
			Host:  newH.Host,
			Label: newH.Label,
		}

		if oldH, ok := oldest.Hosts[host]; ok {
			delta.Requests = nonNegative(newH.Requests - oldH.Requests)
			delta.RequestsIn = nonNegative(newH.RequestsIn - oldH.RequestsIn)
			delta.ResponsesOut = nonNegative(newH.ResponsesOut - oldH.ResponsesOut)
			delta.StatusCodes = make(map[string]float64)
			for cls, v := range newH.StatusCodes {
				delta.StatusCodes[cls] = nonNegative(v - oldH.StatusCodes[cls])
			}
		} else {
			// Host was not present at the start of the window; use all its
			// traffic since the first snapshot that recorded it.
			delta.Requests = newH.Requests
			delta.RequestsIn = newH.RequestsIn
			delta.ResponsesOut = newH.ResponsesOut
			delta.StatusCodes = newH.StatusCodes
		}

		result.Hosts = append(result.Hosts, delta)
	}

	return result
}

// Start launches the background goroutine that polls the supplied fetchFn
// every snapshotInterval and flushes to disk every flushInterval.
// It returns immediately; the goroutine stops when ctx is cancelled.
//
// fetchFn is typically Manager.snapshotMetrics — it returns a ready-to-store
// MetricsSnapshot.
func (ms *MetricsStore) Start(ctx context.Context, fetchFn func() (*MetricsSnapshot, error)) {
	go ms.run(ctx, fetchFn)
}

// Flush writes current snapshots to disk immediately (e.g. on graceful shutdown).
func (ms *MetricsStore) Flush() {
	if ms.storePath == "" {
		return
	}
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if err := ms.writeLocked(); err != nil {
		log.Printf("metrics_store: flush error: %v", err)
	}
}

// run is the background goroutine body.
func (ms *MetricsStore) run(ctx context.Context, fetchFn func() (*MetricsSnapshot, error)) {
	pollTick := time.NewTicker(snapshotInterval)
	flushTick := time.NewTicker(flushInterval)
	defer pollTick.Stop()
	defer flushTick.Stop()

	// Take an immediate first snapshot so the UI has data quickly.
	if snap, err := fetchFn(); err == nil {
		ms.AddSnapshot(*snap)
	}

	for {
		select {
		case <-ctx.Done():
			ms.Flush()
			return
		case <-pollTick.C:
			snap, err := fetchFn()
			if err != nil {
				log.Printf("metrics_store: poll error: %v", err)
				continue
			}
			ms.AddSnapshot(*snap)
		case <-flushTick.C:
			ms.Flush()
		}
	}
}

// prune removes snapshots older than maxRetention.  Caller must hold mu (write).
func (ms *MetricsStore) prune() {
	if len(ms.snapshots) == 0 {
		return
	}
	cutoff := time.Now().Add(-maxRetention)
	keep := 0
	for i, s := range ms.snapshots {
		if !s.At.Before(cutoff) {
			keep = i
			break
		}
	}
	if keep > 0 {
		ms.snapshots = ms.snapshots[keep:]
	}
}

// load reads the persisted history from disk. Caller must NOT hold mu.
func (ms *MetricsStore) load() error {
	f, err := os.Open(ms.storePath)
	if err != nil {
		return err
	}
	defer f.Close()

	var h metricsHistory
	if err := json.NewDecoder(f).Decode(&h); err != nil {
		return err
	}

	ms.mu.Lock()
	ms.snapshots = h.Snapshots
	ms.prune()
	ms.mu.Unlock()

	log.Printf("metrics_store: loaded %d snapshots from %s", len(h.Snapshots), ms.storePath)
	return nil
}

// writeLocked writes snapshots to disk. Caller must hold mu (at least read).
func (ms *MetricsStore) writeLocked() error {
	tmp := ms.storePath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	if err := enc.Encode(metricsHistory{Snapshots: ms.snapshots}); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()

	return os.Rename(tmp, ms.storePath)
}

// snapshotToMetricsData converts the latest snapshot into a MetricsData value
// used when no window filter is requested.
func snapshotToMetricsData(snap MetricsSnapshot) *MetricsData {
	result := &MetricsData{}
	for _, hm := range snap.Hosts {
		result.Hosts = append(result.Hosts, hm)
	}
	return result
}

// nonNegative clamps a delta to 0 in case of a Caddy restart that reset
// the counter below the baseline we recorded.
func nonNegative(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
