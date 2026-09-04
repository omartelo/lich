package quota

import (
	"testing"
	"time"
)

// paceNow is the clock every case here is written against; markAhead takes it
// as a parameter, which is the whole reason none of this needs a real one.
var paceNow = time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

// resetIn spells a window's next reset as the providers report it: RFC 3339,
// that many seconds after paceNow.
func resetIn(seconds int) string {
	return paceNow.Add(time.Duration(seconds) * time.Second).Format(time.RFC3339)
}

// The boundaries are pinned as literals on purpose, and every case sits one
// step either side of one: a test that reads aheadPoints or aheadGrace keeps
// passing after someone edits them, which is the exact change it exists to
// catch. 604800 is a week, 302400 half of one (so the expected share is a flat
// 50%), and 86400 the grace.
func TestMarkAhead(t *testing.T) {
	tests := []struct {
		name   string
		window Window
		want   bool
	}{{
		name:   "weekly at half its clock, one point under the threshold",
		window: Window{Seconds: 604800, Percent: 64, ResetsAt: resetIn(302400)},
		want:   false,
	}, {
		name:   "weekly at half its clock, exactly on the threshold",
		window: Window{Seconds: 604800, Percent: 65, ResetsAt: resetIn(302400)},
		want:   true,
	}, {
		name:   "weekly at half its clock, one point over the threshold",
		window: Window{Seconds: 604800, Percent: 66, ResetsAt: resetIn(302400)},
		want:   true,
	}, {
		name:   "weekly spending under its own clock",
		window: Window{Seconds: 604800, Percent: 40, ResetsAt: resetIn(302400)},
		want:   false,
	}, {
		name:   "a second short of the grace, spent to the top",
		window: Window{Seconds: 604800, Percent: 100, ResetsAt: resetIn(604800 - 86399)},
		want:   false,
	}, {
		name:   "the grace exactly elapsed, spent to the top",
		window: Window{Seconds: 604800, Percent: 100, ResetsAt: resetIn(604800 - 86400)},
		want:   true,
	}, {
		name:   "the five-hour window is never paced",
		window: Window{Seconds: 18000, Percent: 100, ResetsAt: resetIn(3600)},
		want:   false,
	}, {
		name: "the free tier's monthly window is not weekly, so it is not paced",
		// 2592000 is Codex's 30-day window: past the grace, and half spent at
		// half its clock, so only the weekly-only guard keeps it unmarked.
		window: Window{Seconds: 2592000, Percent: 90, ResetsAt: resetIn(1296000)},
		want:   false,
	}, {
		name:   "a model-scoped weekly cap is paced like the account-wide one",
		window: Window{Label: "Fable", Seconds: 604800, Percent: 65, ResetsAt: resetIn(302400)},
		want:   true,
	}, {
		name:   "a reset three cycles further out still places the current one",
		window: Window{Seconds: 604800, Percent: 65, ResetsAt: resetIn(302400 + 3*604800)},
		want:   true,
	}, {
		name:   "a reset already in the past marks nothing",
		window: Window{Seconds: 604800, Percent: 100, ResetsAt: resetIn(-3600)},
		want:   false,
	}, {
		name:   "no reset time, nothing to pace against",
		window: Window{Seconds: 604800, Percent: 100},
		want:   false,
	}, {
		name:   "an unparseable reset time marks nothing",
		window: Window{Seconds: 604800, Percent: 100, ResetsAt: "next tuesday"},
		want:   false,
	}, {
		name:   "a window whose length the provider never reported",
		window: Window{Percent: 100, ResetsAt: resetIn(302400)},
		want:   false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := markAhead(tt.window, paceNow)
			if got.Ahead != tt.want {
				t.Errorf("Ahead = %v, want %v", got.Ahead, tt.want)
			}
			got.Ahead = tt.window.Ahead
			if got != tt.window {
				t.Errorf("markAhead changed more than the mark: %+v, want %+v", got, tt.window)
			}
		})
	}
}

func TestPaceMarksEveryWindowOfEveryPlan(t *testing.T) {
	plans := []Plan{{
		Provider: "claude",
		Windows: []Window{
			{Label: "Session", Seconds: 18000, Percent: 90, ResetsAt: resetIn(3600)},
			{Label: "Weekly", Seconds: 604800, Percent: 66, ResetsAt: resetIn(302400)},
		},
	}, {
		Provider: "codex",
		Windows: []Window{
			{Label: "Weekly", Seconds: 604800, Percent: 40, ResetsAt: resetIn(302400)},
		},
	}}
	pace(plans, paceNow)

	want := []bool{false, true, false}
	got := []bool{
		plans[0].Windows[0].Ahead,
		plans[0].Windows[1].Ahead,
		plans[1].Windows[0].Ahead,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("marks = %v, want %v", got, want)
		}
	}
}
