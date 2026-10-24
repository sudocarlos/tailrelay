package logger

import (
	"bytes"
	"strings"
	"testing"
)

// newTestLogger builds a fresh Logger so tests do not depend on the
// sync.Once-guarded global initialized by Init.
func newTestLogger(level Level, output *bytes.Buffer) *Logger {
	return &Logger{
		level:       level,
		output:      output,
		buffer:      NewRingBuffer(1000),
		subscribers: make(map[chan LogEntry]bool),
	}
}

func TestInfoDefaultCapturesRuntimeLogs(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(INFO, &buf)

	l.Info("serve", "Applying relay r1")
	l.Warn("serve", "relay deferred")
	l.Error("serve", "apply failed")

	entries := l.GetHistory()
	if len(entries) != 3 {
		t.Fatalf("expected 3 captured entries, got %d", len(entries))
	}

	wantLevels := []string{"INFO", "WARN", "ERROR"}
	for i, want := range wantLevels {
		if entries[i].Level != want {
			t.Errorf("entry %d: expected level %s, got %s", i, want, entries[i].Level)
		}
	}

	first := entries[0]
	if first.Source != "serve" || first.Message != "Applying relay r1" {
		t.Errorf("unexpected first entry: %+v", first)
	}
}

func TestDebugFilteredAtInfo(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(INFO, &buf)

	l.Debug("serve", "Executing: tailscale serve status --json")
	l.Info("serve", "visible")

	entries := l.GetHistory()
	if len(entries) != 1 {
		t.Fatalf("expected only the INFO entry in history, got %d", len(entries))
	}
	if entries[0].Message != "visible" {
		t.Errorf("unexpected entry: %+v", entries[0])
	}
	if out := buf.String(); strings.Contains(out, "Executing") {
		t.Errorf("DEBUG entry leaked to output at INFO level: %q", out)
	}
}

func TestDebugCapturedAtDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(DEBUG, &buf)

	l.Debug("serve", "Executing: tailscale serve status --json")
	l.Info("serve", "visible")

	entries := l.GetHistory()
	if len(entries) != 2 {
		t.Fatalf("expected 2 captured entries, got %d", len(entries))
	}
	if entries[0].Level != "DEBUG" {
		t.Errorf("expected DEBUG entry first, got %s", entries[0].Level)
	}
	if out := buf.String(); !strings.Contains(out, "Executing") {
		t.Error("DEBUG entry missing from output at DEBUG level")
	}
}

func TestRingBufferWrapsAround(t *testing.T) {
	var buf bytes.Buffer
	const size = 3
	l := newTestLogger(INFO, &buf)
	l.buffer = NewRingBuffer(size)

	for i := 0; i < size+2; i++ {
		l.Info("test", "entry %d", i)
	}

	entries := l.GetHistory()
	if len(entries) != size {
		t.Fatalf("expected %d entries, got %d", size, len(entries))
	}
	// Oldest entries are overwritten; the surviving sequence stays chronological.
	if entries[0].Message != "entry 2" || entries[size-1].Message != "entry 4" {
		t.Errorf("unexpected wraparound order: %q, %q", entries[0].Message, entries[size-1].Message)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input   string
		want    Level
		wantErr bool
	}{
		{"DEBUG", DEBUG, false},
		{"INFO", INFO, false},
		{"WARN", WARN, false},
		{"warning", WARN, false},
		{"error", ERROR, false},
		{"", 0, true},
		{"verbose", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseLevel(%q): expected error, got level %d", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseLevel(%q): unexpected error %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseLevel(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
