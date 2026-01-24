package app

import "testing"

func TestSetStatusIfIdle(t *testing.T) {
	m := NewWithOptions(Options{})

	m.statusEpoch = 1
	m.setStatus("first")
	m.setStatusIfIdle("second")
	if m.statusText != "first" {
		t.Fatalf("expected status to remain \"first\", got %q", m.statusText)
	}

	m.statusEpoch = 2
	m.setStatusIfIdle("third")
	if m.statusText != "third" {
		t.Fatalf("expected status to update to \"third\", got %q", m.statusText)
	}
}
