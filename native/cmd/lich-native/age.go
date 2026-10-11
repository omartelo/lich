package main

import (
	"strconv"
	"time"
)

// formatAge is lib/session/session-age.ts: how long a session has been in its
// status, in one floored unit (seconds, then minutes, then hours), so the
// readout never claims time that has not passed. A backwards clock reads as
// none.
//
// ponytail: English narrow units until the native window has i18n.
func formatAge(elapsed time.Duration) string {
	elapsed = max(elapsed, 0)
	switch {
	case elapsed < time.Minute:
		return strconv.Itoa(int(elapsed/time.Second)) + "s"
	case elapsed < time.Hour:
		return strconv.Itoa(int(elapsed/time.Minute)) + "m"
	}
	return strconv.Itoa(int(elapsed/time.Hour)) + "h"
}

// nextAgeChange is how long until formatAge(elapsed) reads differently, so a
// frame is asked for when the readout moves rather than on a fixed tick.
func nextAgeChange(elapsed time.Duration) time.Duration {
	elapsed = max(elapsed, 0)
	switch {
	case elapsed < time.Minute:
		return time.Second - elapsed%time.Second
	case elapsed < time.Hour:
		return time.Minute - elapsed%time.Minute
	}
	return time.Hour - elapsed%time.Hour
}
