package doctor

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/singleton"
)

// healthy builds a Doctor over a temp config dir with every probe answering the
// way a working machine does, so each test breaks exactly one thing.
func healthy(t *testing.T) *Doctor {
	t.Helper()
	t.Setenv("LICH_DEV", "")
	d := New(t.TempDir(), func(string) string { return "" })
	d.bind = func(int) error { return nil }
	d.instance = func() (*singleton.Info, bool) { return nil, false }
	d.browser = func() (string, error) { return "/usr/bin/chromium", nil }
	d.detect = func() []providers.Detected {
		return []providers.Detected{
			{ID: "claude", Installed: true, Path: "/usr/bin/claude"},
			{ID: "codex"},
		}
	}
	d.store = func() (io.Closer, error) { return io.NopCloser(nil), nil }
	return d
}

func statuses(checks []Check) map[string]Check {
	out := make(map[string]Check, len(checks))
	for _, c := range checks {
		out[c.Name] = c
	}
	return out
}

func TestAHealthyMachineWalksTheWholeBoot(t *testing.T) {
	checks := healthy(t).Run()

	var names []string
	for _, c := range checks {
		names = append(names, c.Name)
	}
	want := []string{"home", "log", "listener", "store", "browser", "providers"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("checks ran as %v, want the boot order %v", names, want)
	}
	for _, c := range checks {
		if c.Status != OK {
			t.Errorf("%s = %s (%s), want ok", c.Name, c.Status, c.Detail)
		}
	}
	if Failed(checks) {
		t.Error("a healthy machine reports a launch-stopping failure")
	}
}

func TestTheListenerTellsTheRunningLichFromAStranger(t *testing.T) {
	t.Run("held by the running lich", func(t *testing.T) {
		d := healthy(t)
		d.bind = func(int) error { return errors.New("address already in use") }
		d.instance = func() (*singleton.Info, bool) {
			return &singleton.Info{PID: 4242, Port: singleton.DefaultPort, Token: "t"}, true
		}

		got := statuses(d.Run())["listener"]

		if got.Status != OK {
			t.Errorf("listener = %s (%s), want ok — the lock working is not a failure", got.Status, got.Detail)
		}
		if !strings.Contains(got.Detail, "4242") {
			t.Errorf("detail does not name the instance holding it: %q", got.Detail)
		}
	})

	t.Run("held by something else", func(t *testing.T) {
		d := healthy(t)
		d.bind = func(int) error { return errors.New("address already in use") }

		checks := d.Run()
		got := statuses(checks)["listener"]

		if got.Status != Fail {
			t.Errorf("listener = %s (%s), want fail", got.Status, got.Detail)
		}
		if !Failed(checks) {
			t.Error("a port lich cannot bind is not reported as launch-stopping")
		}
	})

	t.Run("a dead instance on another port is not a holder", func(t *testing.T) {
		d := healthy(t)
		d.bind = func(int) error { return errors.New("address already in use") }
		// A live dev instance on its own port does not explain the daily
		// driver's port being busy.
		d.instance = func() (*singleton.Info, bool) {
			return &singleton.Info{PID: 1, Port: singleton.DefaultPort + 1, Token: "t"}, true
		}

		if got := statuses(d.Run())["listener"]; got.Status != Fail {
			t.Errorf("listener = %s (%s), want fail", got.Status, got.Detail)
		}
	})
}

func TestTheStoreIsNeverOpenedBehindARunningInstance(t *testing.T) {
	d := healthy(t)
	d.instance = func() (*singleton.Info, bool) {
		return &singleton.Info{PID: 4242, Port: singleton.DefaultPort, Token: "t"}, true
	}
	opened := false
	d.store = func() (io.Closer, error) {
		opened = true
		return io.NopCloser(nil), nil
	}

	got := statuses(d.Run())["store"]

	if opened {
		t.Error("doctor opened the database a running lich holds — a second migration is not its to run")
	}
	if got.Status != Skip || !strings.Contains(got.Detail, "4242") {
		t.Errorf("store = %s (%s), want skip naming the instance", got.Status, got.Detail)
	}
}

func TestADatabaseThatWillNotOpenStopsALaunch(t *testing.T) {
	d := healthy(t)
	d.store = func() (io.Closer, error) { return nil, errors.New("file is not a database") }

	checks := d.Run()

	if got := statuses(checks)["store"]; got.Status != Fail || !strings.Contains(got.Detail, "not a database") {
		t.Errorf("store = %s (%s), want fail carrying the cause", got.Status, got.Detail)
	}
	if !Failed(checks) {
		t.Error("a database that will not open is not reported as launch-stopping")
	}
}

// A database that opens but will not close is not a launch failure — lich runs
// on it — yet it is the shape of a store about to corrupt, so it warns instead
// of passing silently.
func TestADatabaseThatWillNotCloseWarns(t *testing.T) {
	d := healthy(t)
	d.store = func() (io.Closer, error) {
		return closerFunc(func() error { return errors.New("disk I/O error") }), nil
	}

	checks := d.Run()

	got := statuses(checks)["store"]
	if got.Status != Warn || !strings.Contains(got.Detail, "disk I/O error") {
		t.Errorf("store = %s (%s), want warn carrying the cause", got.Status, got.Detail)
	}
	if Failed(checks) {
		t.Error("a store that opens is reported as launch-stopping")
	}
}

// closerFunc is an io.Closer whose Close is the test's.
type closerFunc func() error

func (c closerFunc) Close() error { return c() }

func TestAMissingBrowserStopsALaunchAndMissingProvidersDoNot(t *testing.T) {
	d := healthy(t)
	d.browser = func() (string, error) { return "", errors.New("no chromium-family browser found") }
	d.detect = func() []providers.Detected { return []providers.Detected{{ID: "claude"}} }

	checks := statuses(d.Run())

	if checks["browser"].Status != Fail {
		t.Errorf("browser = %s, want fail — lich runs and shows nothing", checks["browser"].Status)
	}
	if checks["providers"].Status != Warn {
		t.Errorf("providers = %s, want warn — the window still opens", checks["providers"].Status)
	}
	if !strings.Contains(checks["providers"].Detail, "no session can spawn") {
		t.Errorf("providers detail = %q", checks["providers"].Detail)
	}
}

func TestTheLogIsOpenedWithoutRotatingIt(t *testing.T) {
	d := healthy(t)
	dir := filepath.Join(d.configDir, "lich")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "lich.log")
	if err := os.WriteFile(path, []byte("the line a bug report is about to quote\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := statuses(d.Run())["log"]

	if got.Status != OK || got.Detail != path {
		t.Errorf("log = %s (%s), want ok naming %s", got.Status, got.Detail, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "about to quote") {
		t.Error("doctor moved or truncated the log it was only meant to open")
	}
	if _, err := os.Stat(path + ".old"); err == nil {
		t.Error("doctor rotated the log, taking the generation a bug report attaches")
	}
}

func TestBindProbeGivesThePortStraightBack(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := taken.Addr().(*net.TCPAddr).Port

	if err := bindProbe(port); err == nil {
		t.Error("the probe called a port free while something was listening on it")
	}
	if err := taken.Close(); err != nil {
		t.Fatal(err)
	}
	if err := bindProbe(port); err != nil {
		t.Errorf("the probe refused a free port: %v", err)
	}
	// The launch comes seconds later and has to be able to take it: a probe
	// that kept the port would diagnose the machine into the failure it reports.
	after, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("the probe held the port it proved free: %v", err)
	}
	_ = after.Close()
}

func TestThePortComesFromTheEnvironmentTheLaunchWouldRead(t *testing.T) {
	d := New(t.TempDir(), func(key string) string {
		if key == "LICH_LISTEN_PORT" {
			return "1234"
		}
		return ""
	})
	if d.port != 1234 {
		t.Errorf("port = %d, want the pinned override", d.port)
	}
	if plain := New(t.TempDir(), func(string) string { return "" }); plain.port != singleton.DefaultPort {
		t.Errorf("port = %d, want singleton.DefaultPort", plain.port)
	}
	if nonsense := New(t.TempDir(), func(string) string { return "not-a-port" }); nonsense.port != singleton.DefaultPort {
		t.Errorf("port = %d, want the default when the override is not a number", nonsense.port)
	}
}
