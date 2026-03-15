package caddy

import (
	"testing"
	"time"
)

// makeSnap builds a MetricsSnapshot for testing.
func makeSnap(at time.Time, hosts map[string]HostMetrics) MetricsSnapshot {
	return MetricsSnapshot{At: at, Hosts: hosts}
}

// makeHM builds a minimal HostMetrics for testing.
func makeHM(label string, requests, reqIn, respOut float64) HostMetrics {
	return HostMetrics{
		Label:        label,
		Requests:     requests,
		RequestsIn:   reqIn,
		ResponsesOut: respOut,
		StatusCodes:  map[string]float64{"2xx": requests},
	}
}

// TestMetricsStore_PauseResume verifies that pausing a proxy (RecordPause) and
// then resuming it under a new server key (ApplyBaseline via snapshotMetrics)
// preserves accumulated totals so Query returns the correct all-time value.
func TestMetricsStore_PauseResume(t *testing.T) {
	ms := NewMetricsStore("") // no disk persistence for tests

	const label = ":8080 → backend:3000"

	// Phase 1 — proxy is active on srv0 and accumulates 100 requests.
	t0 := time.Now().Add(-10 * time.Minute)
	ms.AddSnapshot(makeSnap(t0, map[string]HostMetrics{
		"srv0": makeHM(label, 50, 1000, 5000),
	}))
	ms.AddSnapshot(makeSnap(t0.Add(5*time.Minute), map[string]HostMetrics{
		"srv0": makeHM(label, 100, 2000, 10000),
	}))

	// Phase 2 — proxy is paused: RecordPause captures last known value.
	ms.RecordPause(label, makeHM(label, 100, 2000, 10000))

	// Phase 3 — proxy is resumed on srv1: Caddy counters start fresh at 0.
	// ApplyBaseline adds the saved baseline so the stored value stays monotonic.
	resumed := makeHM(label, 0, 0, 0) // raw Caddy value after resume
	adjusted := ms.ApplyBaseline(label, resumed)
	if adjusted.Requests != 100 {
		t.Errorf("expected 100 requests after applying baseline, got %v", adjusted.Requests)
	}
	if adjusted.RequestsIn != 2000 {
		t.Errorf("expected 2000 bytes in after applying baseline, got %v", adjusted.RequestsIn)
	}

	ms.AddSnapshot(makeSnap(time.Now().Add(-1*time.Minute), map[string]HostMetrics{
		"srv1": adjusted,
	}))

	// Phase 4 — some new traffic arrives: 5 more requests in Caddy.
	newTraffic := makeHM(label, 5, 100, 500)
	adjusted2 := ms.ApplyBaseline(label, newTraffic)
	if adjusted2.Requests != 105 {
		t.Errorf("expected 105 total requests after new traffic, got %v", adjusted2.Requests)
	}

	ms.AddSnapshot(makeSnap(time.Now(), map[string]HostMetrics{
		"srv1": adjusted2,
	}))

	// Query(0) — all-time: newest srv1 value should be 105.
	result := ms.Query(0)
	found := false
	for _, hm := range result.Hosts {
		if hm.Label == label {
			found = true
			if hm.Requests != 105 {
				t.Errorf("all-time query: expected 105 requests, got %v", hm.Requests)
			}
		}
	}
	if !found {
		t.Errorf("all-time query: label %q not found in result", label)
	}
}

// TestMetricsStore_RecordPause_Accumulates verifies that multiple
// pause/resume cycles accumulate correctly (baseline stacks).
func TestMetricsStore_RecordPause_Accumulates(t *testing.T) {
	ms := NewMetricsStore("")

	const label = ":9090 → svc:80"

	// First pause: 50 requests accumulated.
	ms.RecordPause(label, makeHM(label, 50, 0, 0))

	// Second pause (another cycle): 30 more requests in Caddy since last resume.
	ms.RecordPause(label, makeHM(label, 30, 0, 0))

	// After two pauses, baseline should be 80.
	raw := makeHM(label, 5, 0, 0) // Caddy counter at 5 after second resume
	adjusted := ms.ApplyBaseline(label, raw)
	if adjusted.Requests != 85 {
		t.Errorf("expected 85 requests after two pause cycles, got %v", adjusted.Requests)
	}
}

// TestMetricsStore_LastSnapshot returns nil on empty store and the latest snapshot otherwise.
func TestMetricsStore_LastSnapshot(t *testing.T) {
	ms := NewMetricsStore("")

	if ms.LastSnapshot() != nil {
		t.Fatal("expected nil from empty store")
	}

	t0 := time.Now().Add(-1 * time.Minute)
	ms.AddSnapshot(makeSnap(t0, map[string]HostMetrics{"srv0": makeHM(":80 → a:80", 10, 0, 0)}))
	ms.AddSnapshot(makeSnap(time.Now(), map[string]HostMetrics{"srv0": makeHM(":80 → a:80", 20, 0, 0)}))

	last := ms.LastSnapshot()
	if last == nil {
		t.Fatal("expected non-nil last snapshot")
	}
	if last.Hosts["srv0"].Requests != 20 {
		t.Errorf("expected 20 requests in last snapshot, got %v", last.Hosts["srv0"].Requests)
	}
}

// TestMetricsStore_GlobalReset simulates the Caddy behaviour where toggling any
// proxy resets ALL HTTP metric counters to 0.  The snapshotMetrics caller is
// expected to detect the reset, call RecordPause for every label, and then
// apply baselines so stored values remain monotonically increasing.
//
// This test exercises the store primitives (RecordPause + ApplyBaseline) in the
// same sequence that snapshotMetrics uses them; the full integration is covered
// by the manager-level test.
func TestMetricsStore_GlobalReset(t *testing.T) {
	ms := NewMetricsStore("")

	const (
		labelA = ":8080 → backend-a:80"
		labelB = ":9090 → backend-b:80"
	)

	// Pre-reset: two relays have accumulated traffic.
	t0 := time.Now().Add(-5 * time.Minute)
	ms.AddSnapshot(makeSnap(t0, map[string]HostMetrics{
		"srv0": makeHM(labelA, 100, 2000, 10000),
		"srv1": makeHM(labelB, 50, 1000, 5000),
	}))

	prev := ms.LastSnapshot()

	// Simulate reset detection: save baselines for all active relays.
	for _, prevHM := range prev.Hosts {
		if prevHM.Label != "" {
			ms.RecordPause(prevHM.Label, prevHM)
		}
	}

	// Caddy counters reset to 0 for both servers.
	rawA := makeHM(labelA, 0, 0, 0)
	rawB := makeHM(labelB, 0, 0, 0)

	adjustedA := ms.ApplyBaseline(labelA, rawA)
	adjustedB := ms.ApplyBaseline(labelB, rawB)

	ms.AddSnapshot(makeSnap(time.Now(), map[string]HostMetrics{
		"srv0": adjustedA,
		"srv1": adjustedB,
	}))

	// window=0: should return pre-reset totals, not zeros.
	result := ms.Query(0)
	byLabel := map[string]HostMetrics{}
	for _, hm := range result.Hosts {
		byLabel[hm.Label] = hm
	}

	if byLabel[labelA].Requests != 100 {
		t.Errorf("labelA: expected 100 after reset, got %v", byLabel[labelA].Requests)
	}
	if byLabel[labelB].Requests != 50 {
		t.Errorf("labelB: expected 50 after reset, got %v", byLabel[labelB].Requests)
	}

	// New traffic after reset: 5 more requests on A.
	rawA2 := makeHM(labelA, 5, 100, 500)
	adjustedA2 := ms.ApplyBaseline(labelA, rawA2)
	ms.AddSnapshot(makeSnap(time.Now().Add(time.Second), map[string]HostMetrics{
		"srv0": adjustedA2,
		"srv1": adjustedB, // B unchanged
	}))

	result2 := ms.Query(0)
	byLabel2 := map[string]HostMetrics{}
	for _, hm := range result2.Hosts {
		byLabel2[hm.Label] = hm
	}

	if byLabel2[labelA].Requests != 105 {
		t.Errorf("labelA after new traffic: expected 105, got %v", byLabel2[labelA].Requests)
	}
	if byLabel2[labelB].Requests != 50 {
		t.Errorf("labelB: expected 50 (unchanged), got %v", byLabel2[labelB].Requests)
	}
}

// TestMetricsStore_Query_Window verifies windowed delta queries still work
// correctly when all snapshots are on the same server key (no pause).
func TestMetricsStore_Query_Window(t *testing.T) {
	ms := NewMetricsStore("")

	const label = ":7777 → db:5432"
	now := time.Now()

	ms.AddSnapshot(makeSnap(now.Add(-2*time.Hour), map[string]HostMetrics{
		"srv0": makeHM(label, 100, 0, 0),
	}))
	ms.AddSnapshot(makeSnap(now.Add(-30*time.Minute), map[string]HostMetrics{
		"srv0": makeHM(label, 150, 0, 0),
	}))
	ms.AddSnapshot(makeSnap(now, map[string]HostMetrics{
		"srv0": makeHM(label, 200, 0, 0),
	}))

	// 1-hour window: newest(200) - oldest_in_window(150) = 50.
	result := ms.Query(time.Hour)
	for _, hm := range result.Hosts {
		if hm.Label == label {
			if hm.Requests != 50 {
				t.Errorf("1h window: expected 50 requests, got %v", hm.Requests)
			}
		}
	}

	// All-time (window=0): returns newest raw value = 200.
	result = ms.Query(0)
	for _, hm := range result.Hosts {
		if hm.Label == label {
			if hm.Requests != 200 {
				t.Errorf("all-time: expected 200 requests, got %v", hm.Requests)
			}
		}
	}
}
