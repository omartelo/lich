package main

import (
	"testing"
	"time"
)

// Mirrors frontend/src/lib/session/session-age.test.ts.
func TestFormatAge(t *testing.T) {
	cases := []struct {
		elapsed time.Duration
		want    string
		next    time.Duration
	}{
		{-5 * time.Second, "0s", time.Second},
		{0, "0s", time.Second},
		{59*time.Second + 999*time.Millisecond, "59s", time.Millisecond},
		{time.Minute, "1m", time.Minute},
		{59*time.Minute + 30*time.Second, "59m", 30 * time.Second},
		{time.Hour, "1h", time.Hour},
		{26*time.Hour + 15*time.Minute, "26h", 45 * time.Minute},
	}
	for _, c := range cases {
		if got := formatAge(c.elapsed); got != c.want {
			t.Errorf("formatAge(%v) = %q, want %q", c.elapsed, got, c.want)
		}
		if got := nextAgeChange(c.elapsed); got != c.next {
			t.Errorf("nextAgeChange(%v) = %v, want %v", c.elapsed, got, c.next)
		}
	}
}
