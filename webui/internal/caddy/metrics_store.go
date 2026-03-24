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
// Hosts contains raw Caddy counter values only — no baselines are baked in.
// Baselines is a copy of the active baseline map at snapshot time, used by
// Query to apply the correct per-snapshot baseline offset when computing
// windowed deltas across pause/resume boundaries.
type MetricsSnapshot struct {
	At        time.Time                `json:"at"`
	Hosts     map[string]HostMetrics   `json:"hosts"`               // raw Caddy counters
	Baselines map[string]labelBaseline `json:"baselines,omitempty"` // baseline state at this moment
}

// metricsHistory is the on-disk/wire format for the full snapshot list,
// accumulated baselines, and the set of explicitly paused proxy labels.
type metricsHistory struct {
	Snapshots    []MetricsSnapshot        `json:"snapshots"`
	Baselines    map[string]labelBaseline `json:"baselines,omitempty"`
	PausedLabels map[string]bool          `json:"paused_labels,omitempty"`
}

// labelBaseline holds the accumulated counter totals for one proxy label at
// the moment its Caddy server was last deleted (i.e. when the proxy was paused).
// When the proxy is re-enabled, Caddy creates a fresh server whose counters
// restart from 0.  Adding these baseline values to the new raw counters keeps
// the sequence monotonically increasing so Query() delta math stays correct.
type labelBaseline struct {
	Requests     float64            `json:"requests"`
	RequestsIn   float64            `json:"requests_in"`
	ResponsesOut float64            `json:"responses_out"`
	StatusCodes  map[string]float64 `json:"status_codes"`
}

// MetricsStore accumulates per-host counter snapshots sampled from Caddy and
// answers delta queries over configurable time windows.  It is safe for
// concurrent use.
//
// Storage model: snapshots contain raw Caddy counter values only.  Baselines
// are applied at query time, not at storage time.  Each snapshot carries a
// copy of the baseline map that was active when it was recorded, enabling
// accurate windowed delta queries across pause/resume boundaries:
//
//	delta(window) = (raw_newest + baseline_newest) - (raw_oldest + baseline_oldest)
//
// On-disk persistence: the store periodically writes all snapshots to a JSON
// file so that history survives container restarts.  Snapshots older than
// maxRetention are pruned automatically.
//
// Counter-reset handling: when a proxy is paused its Caddy server is deleted
// and counters reset to 0 on re-enable.  MetricsStore maintains a baselines
// map (keyed by stable proxy label) that records the last-known cumulative
// value each time a proxy disappears.  Because baselines are applied at query
// time rather than baked into stored values, there is no risk of double-
// accumulation across multiple pause/resume cycles.
//
// Paused-proxy tracking: pausedLabels records the set of proxy labels that are
// currently disabled.  Only explicit MarkPaused calls (from ToggleProxy) add
// labels to this set; reset-detection baselines for surviving relays do not.
// AddSnapshot automatically clears a label from pausedLabels when its server
// key reappears in a snapshot, so resuming a proxy is tracked without any
// additional call.
type MetricsStore struct {
	mu        sync.RWMutex
	snapshots []MetricsSnapshot
	storePath string
	// baselines is keyed by proxy label (":port → target"), which is stable
	// across Caddy server renames that occur when a proxy is re-enabled.
	baselines map[string]labelBaseline
	// pausedLabels tracks which proxy labels are currently disabled.
	// Only labels explicitly marked via MarkPaused belong here; surviving
	// relays whose counters were reset by a Caddy config change are NOT paused.
	pausedLabels map[string]bool
}

// NewMetricsStore creates an empty MetricsStore that persists to storePath.
// If storePath is non-empty it attempts to load any existing history from disk.
func NewMetricsStore(storePath string) *MetricsStore {
	ms := &MetricsStore{
		storePath:    storePath,
		baselines:    make(map[string]labelBaseline),
		pausedLabels: make(map[string]bool),
	}
	if storePath != "" {
		if err := ms.load(); err != nil && !os.IsNotExist(err) {
			log.Printf("metrics_store: failed to load history from %s: %v", storePath, err)
		}
	}
	return ms
}

// LastSnapshot returns a pointer to the most recent snapshot, or nil if the
// store is empty.  The returned pointer is only valid while the caller holds
// no lock; copy the value if you need it across a lock boundary.
func (ms *MetricsStore) LastSnapshot() *MetricsSnapshot {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	if len(ms.snapshots) == 0 {
		return nil
	}
	s := ms.snapshots[len(ms.snapshots)-1]
	return &s
}

// AddSnapshot appends a new timestamped snapshot with a copy of the current
// baseline map attached, pruning entries older than maxRetention.
// It also clears any label from pausedLabels whose server key reappears in the
// snapshot, so a resumed proxy is automatically un-paused.
func (ms *MetricsStore) AddSnapshot(snap MetricsSnapshot) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	snap.Baselines = ms.copyBaselinesLocked()
	ms.snapshots = append(ms.snapshots, snap)
	ms.prune()
	// Auto-resume: if a previously paused label now has an active server in the
	// snapshot, remove it from pausedLabels so it is no longer shown as paused.
	if len(ms.pausedLabels) > 0 {
		for _, hm := range snap.Hosts {
			if hm.Label != "" {
				delete(ms.pausedLabels, hm.Label)
			}
		}
	}
}

// copyBaselinesLocked returns a deep copy of the current baselines map.
// Caller must hold mu (write or read lock is both acceptable here since we
// always call this under the write lock from AddSnapshot).
func (ms *MetricsStore) copyBaselinesLocked() map[string]labelBaseline {
	if len(ms.baselines) == 0 {
		return nil
	}
	out := make(map[string]labelBaseline, len(ms.baselines))
	for k, b := range ms.baselines {
		var sc map[string]float64
		if len(b.StatusCodes) > 0 {
			sc = make(map[string]float64, len(b.StatusCodes))
			for code, v := range b.StatusCodes {
				sc[code] = v
			}
		}
		out[k] = labelBaseline{
			Requests:     b.Requests,
			RequestsIn:   b.RequestsIn,
			ResponsesOut: b.ResponsesOut,
			StatusCodes:  sc,
		}
	}
	return out
}

// RecordPause saves the last-known cumulative counters for a proxy label as a
// baseline.  Call this when a proxy is about to be paused (before its Caddy
// server is deleted and its counters disappear).  The baseline is added to raw
// Caddy counters at query time so displayed values stay monotonically increasing.
// RecordPause does NOT mark the label as paused; call MarkPaused separately when
// the disable is explicit (i.e. user-initiated toggle), versus the internal
// reset-detection path for surviving relays.
func (ms *MetricsStore) RecordPause(label string, hm HostMetrics) {
	if label == "" {
		return
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	existing := ms.baselines[label]
	ms.baselines[label] = labelBaseline{
		Requests:     existing.Requests + hm.Requests,
		RequestsIn:   existing.RequestsIn + hm.RequestsIn,
		ResponsesOut: existing.ResponsesOut + hm.ResponsesOut,
		StatusCodes:  mergeStatusCodes(existing.StatusCodes, hm.StatusCodes),
	}
}

// MarkPaused records that the proxy with the given label has been explicitly
// disabled by the user.  Query will include a Paused=true entry for this label
// until its server reappears in a snapshot (auto-cleared by AddSnapshot).
func (ms *MetricsStore) MarkPaused(label string) {
	if label == "" {
		return
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.pausedLabels[label] = true
}

// ApplyBaseline returns a copy of hm with the stored baseline for label added
// to all counter fields.  Used at query time to produce display-ready values.
func (ms *MetricsStore) ApplyBaseline(label string, hm HostMetrics) HostMetrics {
	ms.mu.RLock()
	b, ok := ms.baselines[label]
	ms.mu.RUnlock()
	if !ok {
		return hm
	}
	return applyBaselineEntry(b, hm)
}

// applyBaselineEntry adds a labelBaseline to a HostMetrics value without
// acquiring any lock.  Used by Query which already holds the read lock.
func applyBaselineEntry(b labelBaseline, hm HostMetrics) HostMetrics {
	out := hm
	out.Requests += b.Requests
	out.RequestsIn += b.RequestsIn
	out.ResponsesOut += b.ResponsesOut
	if len(b.StatusCodes) > 0 {
		merged := make(map[string]float64, len(hm.StatusCodes)+len(b.StatusCodes))
		for k, v := range hm.StatusCodes {
			merged[k] = v
		}
		for k, v := range b.StatusCodes {
			merged[k] += v
		}
		out.StatusCodes = merged
	}
	return out
}

// mergeStatusCodes returns the element-wise sum of a and b.
func mergeStatusCodes(a, b map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] += v
	}
	return out
}

// Query computes the delta of all host counters over the given window duration,
// with baselines applied at read time so results survive pause/resume cycles.
//
// window == 0 returns the absolute latest snapshot values (raw + current baseline).
//
// For window > 0 the returned value per host is:
//
//	(raw_newest + baseline_newest) - (raw_oldest_in_window + baseline_oldest_in_window)
//
// Matching between newest and oldest snapshots is done by proxy label (not by
// Caddy server name) so that a proxy that changed server names after a
// pause/resume is still correctly matched across the two ends of the window.
//
// Paused proxies — those whose Caddy server has been deleted but whose baseline
// is non-zero — are included in the result with Paused=true and raw counters of
// zero so that historical totals remain visible in the UI.
func (ms *MetricsStore) Query(window time.Duration) *MetricsData {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if len(ms.snapshots) == 0 {
		return &MetricsData{}
	}

	newest := ms.snapshots[len(ms.snapshots)-1]

	if window == 0 {
		result := snapshotToMetricsData(newest, ms.baselines)

		// Collect labels already present in the result (active proxies).
		emitted := make(map[string]bool, len(result.Hosts))
		for _, hm := range result.Hosts {
			if hm.Label != "" {
				emitted[hm.Label] = true
			}
		}

		// Emit a paused entry for every explicitly paused label not covered by
		// an active host in the latest snapshot.
		for label := range ms.pausedLabels {
			if emitted[label] {
				continue
			}
			b := ms.baselines[label]
			if b.Requests == 0 && b.RequestsIn == 0 && b.ResponsesOut == 0 {
				continue // nothing to show yet
			}
			hm := applyBaselineEntry(b, HostMetrics{Label: label, Paused: true})
			result.Hosts = append(result.Hosts, hm)
		}

		// Emit an active entry for every label that has a non-zero baseline but
		// is absent from the latest snapshot and is NOT explicitly paused.
		// This happens when Prometheus omits all-zero counter lines for a server
		// that is technically still active (e.g. immediately after a Caddy reset
		// before any new traffic arrives).  Without this pass those relays would
		// silently disappear from the UI until their first non-zero request.
		for label, b := range ms.baselines {
			if emitted[label] {
				continue
			}
			if ms.pausedLabels[label] {
				continue // already handled above
			}
			if b.Requests == 0 && b.RequestsIn == 0 && b.ResponsesOut == 0 {
				continue
			}
			hm := applyBaselineEntry(b, HostMetrics{Label: label})
			result.Hosts = append(result.Hosts, hm)
		}

		return result
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

	// Build a label → adjusted-value index for the oldest snapshot.
	// This allows matching across server name changes that occur on pause/resume.
	oldByLabel := make(map[string]HostMetrics, len(oldest.Hosts))
	for _, oldH := range oldest.Hosts {
		if oldH.Label != "" {
			oldByLabel[oldH.Label] = applyBaselineEntry(oldest.Baselines[oldH.Label], oldH)
		}
	}

	// Delta: for every host in the newest snapshot compute:
	//   (raw_new + baseline_new) - (raw_old + baseline_old)
	// Match on label first; fall back to same server key if label is empty.
	result := &MetricsData{}
	emitted := make(map[string]bool)

	for host, newH := range newest.Hosts {
		delta := HostMetrics{
			Host:  newH.Host,
			Label: newH.Label,
		}

		// Apply newest snapshot's baseline to the new raw value.
		adjustedNew := applyBaselineEntry(newest.Baselines[newH.Label], newH)

		// Try to find the matching old value by label (survives server renames).
		if newH.Label != "" {
			emitted[newH.Label] = true
			if adjustedOld, ok := oldByLabel[newH.Label]; ok {
				delta.Requests = nonNegative(adjustedNew.Requests - adjustedOld.Requests)
				delta.RequestsIn = nonNegative(adjustedNew.RequestsIn - adjustedOld.RequestsIn)
				delta.ResponsesOut = nonNegative(adjustedNew.ResponsesOut - adjustedOld.ResponsesOut)
				delta.StatusCodes = make(map[string]float64)
				for cls, v := range adjustedNew.StatusCodes {
					delta.StatusCodes[cls] = nonNegative(v - adjustedOld.StatusCodes[cls])
				}
				result.Hosts = append(result.Hosts, delta)
				continue
			}
		}

		// Fallback: same server key in oldest snapshot (label-less or new proxy).
		if oldH, ok := oldest.Hosts[host]; ok {
			adjustedOld := applyBaselineEntry(oldest.Baselines[oldH.Label], oldH)
			delta.Requests = nonNegative(adjustedNew.Requests - adjustedOld.Requests)
			delta.RequestsIn = nonNegative(adjustedNew.RequestsIn - adjustedOld.RequestsIn)
			delta.ResponsesOut = nonNegative(adjustedNew.ResponsesOut - adjustedOld.ResponsesOut)
			delta.StatusCodes = make(map[string]float64)
			for cls, v := range adjustedNew.StatusCodes {
				delta.StatusCodes[cls] = nonNegative(v - adjustedOld.StatusCodes[cls])
			}
		} else {
			// Host was not present at the start of the window; use all its
			// traffic since the first snapshot that recorded it.
			delta.Requests = adjustedNew.Requests
			delta.RequestsIn = adjustedNew.RequestsIn
			delta.ResponsesOut = adjustedNew.ResponsesOut
			delta.StatusCodes = adjustedNew.StatusCodes
		}

		result.Hosts = append(result.Hosts, delta)
	}

	// Paused proxies: emit entries for labels that are explicitly paused and
	// have no active server in the newest snapshot.
	// adjustedNew = baseline only (raw = 0, server is gone).
	// adjustedOld = oldByLabel[label] if the proxy was active at window start.
	for label := range ms.pausedLabels {
		if emitted[label] {
			continue
		}
		b, ok := newest.Baselines[label]
		if !ok {
			b = ms.baselines[label] // fall back to live baseline if snapshot copy absent
		}
		if b.Requests == 0 && b.RequestsIn == 0 && b.ResponsesOut == 0 {
			continue
		}
		adjustedNew := HostMetrics{
			Label:        label,
			Requests:     b.Requests,
			RequestsIn:   b.RequestsIn,
			ResponsesOut: b.ResponsesOut,
			StatusCodes:  b.StatusCodes,
		}
		delta := HostMetrics{Label: label, Paused: true}
		if adjustedOld, ok := oldByLabel[label]; ok {
			delta.Requests = nonNegative(adjustedNew.Requests - adjustedOld.Requests)
			delta.RequestsIn = nonNegative(adjustedNew.RequestsIn - adjustedOld.RequestsIn)
			delta.ResponsesOut = nonNegative(adjustedNew.ResponsesOut - adjustedOld.ResponsesOut)
			delta.StatusCodes = make(map[string]float64)
			for cls, v := range adjustedNew.StatusCodes {
				delta.StatusCodes[cls] = nonNegative(v - adjustedOld.StatusCodes[cls])
			}
		} else {
			delta.Requests = adjustedNew.Requests
			delta.RequestsIn = adjustedNew.RequestsIn
			delta.ResponsesOut = adjustedNew.ResponsesOut
			delta.StatusCodes = adjustedNew.StatusCodes
		}
		result.Hosts = append(result.Hosts, delta)
	}

	// Active-but-absent proxies: emit entries for labels that have a non-zero
	// baseline, are absent from the newest snapshot, and are NOT explicitly
	// paused.  Prometheus omits counter lines entirely for servers with all-zero
	// values, so a relay that is technically active but has had no traffic since
	// a Caddy reset will disappear from the snapshot.  We synthesise an entry
	// using the baseline alone so the UI continues to show the historical total.
	for label, b := range ms.baselines {
		if emitted[label] {
			continue
		}
		if ms.pausedLabels[label] {
			continue // already handled above
		}
		if b.Requests == 0 && b.RequestsIn == 0 && b.ResponsesOut == 0 {
			continue
		}
		// Use the per-snapshot baseline copy if available, otherwise live.
		snapB, ok := newest.Baselines[label]
		if !ok {
			snapB = b
		}
		adjustedNew := HostMetrics{
			Label:        label,
			Requests:     snapB.Requests,
			RequestsIn:   snapB.RequestsIn,
			ResponsesOut: snapB.ResponsesOut,
			StatusCodes:  snapB.StatusCodes,
		}
		delta := HostMetrics{Label: label}
		if adjustedOld, ok := oldByLabel[label]; ok {
			delta.Requests = nonNegative(adjustedNew.Requests - adjustedOld.Requests)
			delta.RequestsIn = nonNegative(adjustedNew.RequestsIn - adjustedOld.RequestsIn)
			delta.ResponsesOut = nonNegative(adjustedNew.ResponsesOut - adjustedOld.ResponsesOut)
			delta.StatusCodes = make(map[string]float64)
			for cls, v := range adjustedNew.StatusCodes {
				delta.StatusCodes[cls] = nonNegative(v - adjustedOld.StatusCodes[cls])
			}
		} else {
			delta.Requests = adjustedNew.Requests
			delta.RequestsIn = adjustedNew.RequestsIn
			delta.ResponsesOut = adjustedNew.ResponsesOut
			delta.StatusCodes = adjustedNew.StatusCodes
		}
		result.Hosts = append(result.Hosts, delta)
	}

	return result
}

// ResetMetrics clears all stored snapshots and baselines, effectively zeroing
// all displayed counters.  The on-disk history file is overwritten immediately.
func (ms *MetricsStore) ResetMetrics() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.snapshots = nil
	ms.baselines = make(map[string]labelBaseline)
	ms.pausedLabels = make(map[string]bool)
	if ms.storePath != "" {
		if err := ms.writeLocked(); err != nil {
			log.Printf("metrics_store: failed to write after reset: %v", err)
		}
	}
}

// Start launches the background goroutine that polls the supplied fetchFn
// every snapshotInterval and flushes to disk every flushInterval.
// It returns immediately; the goroutine stops when ctx is cancelled.
//
// fetchFn is typically Manager.snapshotMetrics — it returns a ready-to-store
// MetricsSnapshot (raw Caddy values; baselines attached by AddSnapshot).
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
	if h.Baselines != nil {
		ms.baselines = h.Baselines
	}
	if h.PausedLabels != nil {
		ms.pausedLabels = h.PausedLabels
	}
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
	if err := enc.Encode(metricsHistory{
		Snapshots:    ms.snapshots,
		Baselines:    ms.baselines,
		PausedLabels: ms.pausedLabels,
	}); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()

	return os.Rename(tmp, ms.storePath)
}

// snapshotToMetricsData converts the latest snapshot into a MetricsData value,
// applying the provided baselines map to each host so callers see adjusted totals.
func snapshotToMetricsData(snap MetricsSnapshot, baselines map[string]labelBaseline) *MetricsData {
	result := &MetricsData{}
	for _, hm := range snap.Hosts {
		if b, ok := baselines[hm.Label]; ok && hm.Label != "" {
			hm = applyBaselineEntry(b, hm)
		}
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
