package main

import (
	"testing"
	"time"
)

func TestRandomWatchdogInterval_InRange(t *testing.T) {
	for range 200 {
		d := randomWatchdogInterval()
		if d < 1*time.Minute || d > 5*time.Minute {
			t.Errorf("interval %v out of expected range [1m, 5m]", d)
		}
	}
}

func TestRandomWatchdogInterval_NotAlwaysSame(t *testing.T) {
	seen := make(map[time.Duration]bool)
	for range 50 {
		seen[randomWatchdogInterval()] = true
	}
	if len(seen) < 2 {
		t.Error("expected multiple distinct intervals, got only one value — randomness may be broken")
	}
}