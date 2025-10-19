package timeutil

import (
	"testing"
	"time"
)

func TestParseWindowDefault(t *testing.T) {
	dur, label, err := ParseWindow("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := 7 * 24 * time.Hour
	if dur != want {
		t.Fatalf("expected %v, got %v", want, dur)
	}
	if label != "1w" {
		t.Fatalf("expected label 1w, got %s", label)
	}
}

func TestParseWindowComposite(t *testing.T) {
	dur, label, err := ParseWindow("1w2d6h30m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := (7*24+2*24+6)*time.Hour + 30*time.Minute
	if dur != want {
		t.Fatalf("expected %v, got %v", want, dur)
	}
	if label != "1w2d6h30m" {
		t.Fatalf("unexpected label: %s", label)
	}
}

func TestParseWindowInvalid(t *testing.T) {
	if _, _, err := ParseWindow("noop"); err == nil {
		t.Fatalf("expected error for invalid window")
	}
}
