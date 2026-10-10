package chromium

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestEndingOf(t *testing.T) {
	crash := errors.New("exit status 139")
	cases := []struct {
		name        string
		err         error
		first       bool
		uptime      time.Duration
		tabFallback bool
		want        Ending
	}{
		{"closed by the user", nil, true, time.Second, false, WindowClosed},
		{"closed by the user on a Mac", nil, true, time.Second, true, WindowClosed},
		{"no shell on a Mac opens a tab", ErrNoShell, true, 0, true, WindowTabInstead},
		{"wrapped no shell on a Mac opens a tab", fmt.Errorf("launch: %w", ErrNoShell), false, time.Hour, true, WindowTabInstead},
		{"no shell elsewhere never opened", ErrNoShell, true, 0, false, WindowNeverOpened},
		{"first window crashing at startup", crash, true, time.Second, false, WindowNeverOpened},
		{"first window crashing an hour in", crash, true, time.Hour, false, WindowFailed},
		{"reopened window crashing at startup", crash, false, time.Second, false, WindowFailed},
		{"reopened window with no shell elsewhere", ErrNoShell, false, 0, false, WindowFailed},
	}
	for _, c := range cases {
		if got := endingOf(c.err, c.first, c.uptime, c.tabFallback); got != c.want {
			t.Errorf("%s: endingOf = %v, want %v", c.name, got, c.want)
		}
	}
	if got := EndingOf(nil, true, 0); got != WindowClosed {
		t.Errorf("EndingOf(nil) = %v, want WindowClosed", got)
	}
}
