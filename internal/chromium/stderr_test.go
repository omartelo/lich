package chromium

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// noTerminal records what it is handed and fails every write, the stderr of a
// launcher start.
type noTerminal struct{ got []byte }

func (n *noTerminal) Write(p []byte) (int, error) {
	n.got = append(n.got, p...)
	return 0, errors.New("no terminal")
}

func write(t *testing.T, tail *stderrTail, chunks ...string) {
	t.Helper()
	for _, chunk := range chunks {
		if n, err := tail.Write([]byte(chunk)); n != len(chunk) || err != nil {
			t.Fatalf("Write(%q) = %d, %v; want %d, nil", chunk, n, err, len(chunk))
		}
	}
}

func numbered(prefix string, from, to int) []string {
	var lines []string
	for i := from; i <= to; i++ {
		lines = append(lines, fmt.Sprintf("%s %d", prefix, i))
	}
	return lines
}

// The copy never fails on the terminal: exec would report a window that closed
// cleanly as one that failed.
func TestStderrTailMirrorsEverythingAndNeverFails(t *testing.T) {
	mirror := &noTerminal{}
	tail := &stderrTail{mirror: mirror}
	write(t, tail, "one\n", "tw", "o\r\n\n", "three")
	if got := string(mirror.got); got != "one\ntwo\r\n\nthree" {
		t.Fatalf("mirror got %q", got)
	}
	if got, want := tail.Lines(), []string{"one", "two", "three"}; !slices.Equal(got, want) {
		t.Fatalf("Lines = %q, want %q", got, want)
	}
}

func TestStderrTailKeepsTheLastTwentyLines(t *testing.T) {
	tail := &stderrTail{mirror: io.Discard}
	for _, line := range numbered("noise", 1, 25) {
		write(t, tail, line+"\n")
	}
	want := append([]string{"..."}, numbered("noise", 6, 25)...)
	if got := tail.Lines(); !slices.Equal(got, want) {
		t.Fatalf("Lines = %q, want %q", got, want)
	}
}

// The first FATAL lines survive what follows them, with a gap wherever lines
// around them were dropped.
func TestStderrTailKeepsTheFirstFatalLinesThatScrollOut(t *testing.T) {
	tail := &stderrTail{mirror: io.Discard}
	before := []string{
		"Failed to connect to the bus",
		"[1:1:0911/1:FATAL:zygote_host_impl_linux.cc:127] first",
		"Check failed: second",
		"Fontconfig warning",
		"[1:1:0911/1:FATAL:gpu.cc:3] third",
		"[1:1:0911/1:FATAL:gpu.cc:4] fourth",
	}
	write(t, tail, strings.Join(before, "\n")+"\n")
	for _, line := range numbered("after", 1, 20) {
		write(t, tail, line+"\n")
	}
	want := append([]string{"...", before[1], before[2], "...", before[4], "..."}, numbered("after", 1, 20)...)
	if got := tail.Lines(); !slices.Equal(got, want) {
		t.Fatalf("Lines = %q, want %q", got, want)
	}
}

func TestStderrTailCutsALineThatNeverEnds(t *testing.T) {
	tail := &stderrTail{mirror: io.Discard}
	write(t, tail, strings.Repeat("x", 1000), strings.Repeat("y", 1000), "\nnext")
	got := tail.Lines()
	if want := strings.Repeat("x", 1000) + strings.Repeat("y", 24); len(got) != 2 || got[0] != want || got[1] != "next" {
		t.Fatalf("Lines = %d lines, first %d bytes; want a 1024-byte line then next", len(got), len(got[0]))
	}
}

func TestStderrTailExit(t *testing.T) {
	tail := &stderrTail{mirror: io.Discard}
	write(t, tail, "lich-shell: gone\n")
	if err := tail.exit(nil); err != nil {
		t.Fatalf("clean exit = %v, want nil", err)
	}
	// A subprocess holding the pipe past the drain is still a window the user closed.
	if err := tail.exit(fmt.Errorf("wait: %w", exec.ErrWaitDelay)); err != nil {
		t.Fatalf("drain overrun = %v, want nil", err)
	}
	died := errors.New("signal: trace/breakpoint trap (core dumped)")
	var exit *ExitError
	if err := tail.exit(died); !errors.As(err, &exit) || !errors.Is(err, died) {
		t.Fatalf("failed exit = %#v, want an ExitError wrapping the exit", err)
	}
	if want := []string{"lich-shell: gone"}; !slices.Equal(exit.Stderr, want) {
		t.Fatalf("Stderr = %q, want %q", exit.Stderr, want)
	}
	if got, want := exit.Error(), "signal: trace/breakpoint trap (core dumped)\nlich-shell: gone"; got != want {
		t.Fatalf("Error = %q, want %q", got, want)
	}
}

func TestWhy(t *testing.T) {
	died := errors.New("signal: trace/breakpoint trap (core dumped)")
	sandbox := "[1:1:0911/1:FATAL:setuid_sandbox_host.cc:166] The SUID sandbox helper binary was found"
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"not the window's exit", errors.New("launch lich-shell: permission denied"), "launch lich-shell: permission denied"},
		{"nothing on stderr", &ExitError{Err: died}, died.Error()},
		{
			"the FATAL line over the noise around it",
			fmt.Errorf("focus: %w", &ExitError{Err: died, Stderr: []string{"Fontconfig warning", sandbox, "[2:2:0911/1:ERROR:bus.cc:407] Failed"}}),
			died.Error() + "\n" + sandbox,
		},
		{
			"the first three FATAL lines",
			&ExitError{Err: died, Stderr: []string{"...", ":FATAL: a", "Check failed: b", "noise", ":FATAL: c", ":FATAL: d"}},
			died.Error() + "\n:FATAL: a\nCheck failed: b\n:FATAL: c",
		},
		{
			"the last three lines when nothing was fatal",
			&ExitError{Err: died, Stderr: numbered("line", 1, 5)},
			died.Error() + "\nline 3\nline 4\nline 5",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Why(tt.err); got != tt.want {
				t.Fatalf("Why = %q, want %q", got, tt.want)
			}
		})
	}
}
