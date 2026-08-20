package ui

import (
	"testing"
	"time"
)

func TestETANeedsMinimumData(t *testing.T) {
	e := NewETA()
	if _, ok := e.Update(100, 1000); ok {
		t.Fatal("ETA must not report an estimate immediately")
	}
}

func TestETAEstimate(t *testing.T) {
	e := NewETA()
	// Fake a steady 100 bytes/s by back-dating the first sample.
	e.samples = []etaSample{{time.Now().Add(-10 * time.Second), 0}}
	secs, ok := e.Update(1000, 5000) // 1000 bytes in 10s -> 100 B/s, 4000 left
	if !ok {
		t.Fatal("expected an estimate")
	}
	if secs < 35 || secs > 45 {
		t.Fatalf("estimate %f, want ~40s", secs)
	}
}

func TestETAStalled(t *testing.T) {
	e := NewETA()
	e.samples = []etaSample{{time.Now().Add(-10 * time.Second), 500}}
	if _, ok := e.Update(500, 1000); ok {
		t.Fatal("no estimate when no bytes flow")
	}
}

func TestETADoneClampsToZero(t *testing.T) {
	e := NewETA()
	e.samples = []etaSample{{time.Now().Add(-10 * time.Second), 0}}
	secs, ok := e.Update(2000, 1000) // more done than total (file grew)
	if !ok || secs != 0 {
		t.Fatalf("got %f/%v, want 0/true", secs, ok)
	}
}
