package doctor

import (
	"fmt"
	"io"
	"runtime"
	"time"
)

// Render writes the report: a header naming the build, one line per check in
// boot order, the total, and a verdict. The verdict is the whole point of
// reading it — a table of six lines that never says whether lich would start
// leaves the reader to add it up.
func Render(w io.Writer, version string, checks []Check) {
	fmt.Fprintf(w, "lich %s — %s/%s\n\n", version, runtime.GOOS, runtime.GOARCH)

	var total time.Duration
	for _, c := range checks {
		total += c.Took
		fmt.Fprintf(w, "  %-5s %-10s %7s  %s\n", c.Status, c.Name, took(c.Took), c.Detail)
	}
	fmt.Fprintf(w, "  %-5s %-10s %7s\n\n", "", "total", took(total))
	fmt.Fprintln(w, verdict(checks))
}

// verdict is the one line a reader can act on. Failures win over warnings:
// there is no point telling someone a provider is missing when the window will
// not open at all.
func verdict(checks []Check) string {
	failed, warned := 0, 0
	for _, c := range checks {
		switch c.Status {
		case Fail:
			failed++
		case Warn:
			warned++
		}
	}
	switch {
	case failed == 1:
		return "1 check above stops a launch here. Fix it first."
	case failed > 1:
		return fmt.Sprintf("%d checks above stop a launch here. Fix them first.", failed)
	case warned == 1:
		return "lich starts. 1 warning above limits something it offers."
	case warned > 1:
		return fmt.Sprintf("lich starts. %d warnings above limit something it offers.", warned)
	default:
		// Not "every check passed": one of them may have been skipped, and a
		// verdict that overstates what was measured is the one thing a
		// diagnosis may never do.
		return "lich starts here — nothing is in the way."
	}
}

// took renders a duration for a human scanning a column: whole milliseconds,
// and anything faster as a floor rather than a misleading "0s".
func took(d time.Duration) string {
	if d < time.Millisecond {
		return "<1ms"
	}
	return d.Round(time.Millisecond).String()
}
