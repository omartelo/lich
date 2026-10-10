package restart

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	c := New("/usr/local/bin/lich", []string{"PATH=/bin"}, nil)
	if c.exePath != "/usr/local/bin/lich" {
		t.Fatalf("exePath = %q", c.exePath)
	}
	if c.spawn == nil {
		t.Fatal("New left the process primitives unset")
	}
}

func TestSuccessorEnvAppendsWaitMarker(t *testing.T) {
	base := []string{"PATH=/bin", "LICH_LISTEN_PORT=47821"}
	got := successorEnv(base)

	if !slices.Contains(got, WaitEnv+"=1") {
		t.Fatalf("successorEnv = %v, missing %s=1", got, WaitEnv)
	}
	// The base env must survive untouched.
	for _, want := range base {
		if !slices.Contains(got, want) {
			t.Fatalf("successorEnv dropped %q", want)
		}
	}
	// Fresh slice — mutating the result must not touch the caller's env.
	if len(base) != 2 {
		t.Fatalf("base env mutated: %v", base)
	}
}

func TestDoSpawnsThenStops(t *testing.T) {
	var (
		spawnedEnv []string
		order      []string
	)
	c := &Coordinator{
		exePath: "/usr/local/bin/lich",
		env:     []string{"PATH=/bin"},
		spawn: func(_ string, env, _ []string) error {
			spawnedEnv = env
			order = append(order, "spawn")
			return nil
		},
	}
	c.SetStop(func() { order = append(order, "stop") })

	if err := c.Do(); err != nil {
		t.Fatalf("Do() = %v, want nil", err)
	}
	if !slices.Contains(spawnedEnv, WaitEnv+"=1") {
		t.Fatalf("successor env = %v, missing wait marker", spawnedEnv)
	}
	// Successor must start before this lich stops.
	if !slices.Equal(order, []string{"spawn", "stop"}) {
		t.Fatalf("order = %v, want [spawn stop]", order)
	}
}

func TestDoIsIdempotent(t *testing.T) {
	spawns, stops := 0, 0
	c := &Coordinator{
		exePath: "/usr/local/bin/lich",
		spawn:   func(string, []string, []string) error { spawns++; return nil },
	}
	c.SetStop(func() { stops++ })

	for range 3 {
		if err := c.Do(); err != nil {
			t.Fatalf("Do() = %v, want nil", err)
		}
	}
	if spawns != 1 || stops != 1 {
		t.Fatalf("spawns=%d stops=%d, want 1 and 1 — restart must fire once", spawns, stops)
	}
}

func TestDoBeforeServingStillSpawns(t *testing.T) {
	spawned := false
	c := &Coordinator{
		exePath: "/usr/local/bin/lich",
		spawn:   func(string, []string, []string) error { spawned = true; return nil },
	}
	if err := c.Do(); err != nil {
		t.Fatalf("Do() = %v, want nil", err)
	}
	if !spawned {
		t.Fatal("successor was not spawned")
	}
}

func TestQuit(t *testing.T) {
	t.Run("stops without launching anything", func(t *testing.T) {
		stops := 0
		c := &Coordinator{
			exePath: "/usr/local/bin/lich",
			spawn:   func(string, []string, []string) error { t.Fatal("quit launched a process"); return nil },
		}
		c.SetStop(func() { stops++ })
		if err := c.Quit(); err != nil {
			t.Fatalf("Quit() = %v, want nil", err)
		}
		if stops != 1 {
			t.Fatalf("stops = %d, want 1", stops)
		}
	})

	t.Run("refused before lich serves", func(t *testing.T) {
		if err := New("/usr/local/bin/lich", nil, nil).Quit(); err == nil {
			t.Fatal("Quit() = nil before SetStop, want an error")
		}
	})
}

func TestDoErrors(t *testing.T) {
	t.Run("no exe path", func(t *testing.T) {
		c := &Coordinator{spawn: func(string, []string, []string) error { return nil }}
		if err := c.Do(); err == nil {
			t.Fatal("Do() = nil, want error when exe path is unknown")
		}
	})

	t.Run("spawn failure propagates", func(t *testing.T) {
		c := &Coordinator{
			exePath: "/usr/local/bin/lich",
			spawn:   func(string, []string, []string) error { return errors.New("boom") },
		}
		if err := c.Do(); err == nil {
			t.Fatal("Do() = nil, want spawn error")
		}
	})

	t.Run("spawn failure leaves restart retryable", func(t *testing.T) {
		calls := 0
		c := &Coordinator{
			exePath: "/usr/local/bin/lich",
			spawn: func(string, []string, []string) error {
				calls++
				if calls == 1 {
					return errors.New("boom")
				}
				return nil
			},
		}
		if err := c.Do(); err == nil {
			t.Fatal("first Do() = nil, want spawn error")
		}
		if err := c.Do(); err != nil {
			t.Fatalf("second Do() = %v, want a retried spawn to succeed", err)
		}
		if calls != 2 {
			t.Fatalf("spawn calls = %d, want 2 (failure must not latch)", calls)
		}
	})
}

func TestInstallLaunchesBeforeShutdown(t *testing.T) {
	var order []string
	c := New(filepath.Join(t.TempDir(), "lich.exe"), []string{"LICH_LISTEN_PORT=47821"}, nil)
	c.spawn = func(exe string, env, args []string) error {
		if exe != "verified-setup.exe" {
			t.Fatalf("spawned %q instead of installer", exe)
		}
		want := []string{"/SILENT", "/NORESTART", "/CLOSEAPPLICATIONS", "/NORESTARTAPPLICATIONS",
			"/DIR=" + filepath.Dir(c.exePath), "/UPDATEPID=" + strconv.Itoa(os.Getpid()),
			"/LOG=" + filepath.Join(filepath.Dir(c.exePath), ".lich-update-setup.log")}
		if !slices.Equal(args, want) {
			t.Fatalf("installer args = %v, want %v", args, want)
		}
		if !slices.Contains(env, "LICH_LISTEN_PORT=47821") || !slices.Contains(env, WaitEnv+"=1") {
			t.Fatalf("installer lost restart environment: %v", env)
		}
		order = append(order, "launch")
		return nil
	}
	c.SetStop(func() { order = append(order, "shutdown") })
	if err := c.Install("verified-setup.exe"); err != nil {
		t.Fatal(err)
	}
	if err := c.Do(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(order, []string{"launch", "shutdown"}) {
		t.Fatalf("shutdown order = %v", order)
	}
}

func TestInstallerLaunchFailureKeepsRunning(t *testing.T) {
	c := New("lich.exe", nil, nil)
	c.spawn = func(string, []string, []string) error { return io.ErrClosedPipe }
	c.SetStop(func() { t.Fatal("stopped after launch failure") })
	if err := c.Install("setup.exe"); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("launch error = %v", err)
	}
}

// A successor is spawned with the wait marker, and everything it spawns
// inherited it: a session of a restarted lich carried LICH_RESTART_WAIT=1, so a
// `lich` launched from its shell skipped the duplicate check and died on the
// busy port instead of showing the running lich's window.
func TestWithoutMarkerKeepsTheRestartMarkerOutOfChildren(t *testing.T) {
	env := successorEnv([]string{"PATH=/bin", "LICH_LISTEN_PORT=47821"})
	got := WithoutMarker(env)
	if want := []string{"PATH=/bin", "LICH_LISTEN_PORT=47821"}; !slices.Equal(got, want) {
		t.Fatalf("WithoutMarker = %v, want %v", got, want)
	}
	if !slices.Contains(env, WaitEnv+"=1") {
		t.Fatal("WithoutMarker changed the slice it was given")
	}
}

// A successor is lich launched again, and a lich launched with
// `lich -- <chromium flags>` opened its window with them: re-executed with no
// arguments, the restarted window lost them (an --ozone-platform=x11 that
// worked around a driver, gone after the first update).
func TestDoRelaunchesWithTheArgsItWasGiven(t *testing.T) {
	var spawnedArgs []string
	c := New("/usr/local/bin/lich", nil, []string{"--", "--ozone-platform=x11"})
	c.spawn = func(_ string, _ []string, args []string) error {
		spawnedArgs = args
		return nil
	}
	if err := c.Do(); err != nil {
		t.Fatalf("Do() = %v", err)
	}
	if want := []string{"--", "--ozone-platform=x11"}; !slices.Equal(spawnedArgs, want) {
		t.Fatalf("successor args = %v, want %v", spawnedArgs, want)
	}
}

// An update on Windows ends with Inno Setup launching lich again, with a fixed
// command line (build/windows/lich.iss), so the window's switches cannot ride
// it: they ride the environment the installer passes on, as the pinned port
// and the restart marker already do.
func TestInstallHandsTheArgsToTheRelaunchThroughTheEnvironment(t *testing.T) {
	var installerEnv []string
	c := New("lich.exe", []string{"LICH_LISTEN_PORT=47821"}, []string{"--", "--ozone-platform=x11"})
	c.spawn = func(_ string, env, _ []string) error {
		installerEnv = env
		return nil
	}
	if err := c.Install("setup.exe"); err != nil {
		t.Fatal(err)
	}
	got, err := LaunchArgs(nil, envReader(installerEnv))
	if err != nil {
		t.Fatalf("LaunchArgs = %v", err)
	}
	if want := []string{"--", "--ozone-platform=x11"}; !slices.Equal(got, want) {
		t.Fatalf("relaunched lich reads args %v, want %v", got, want)
	}
}

func TestLaunchArgs(t *testing.T) {
	handed := envReader([]string{ArgsEnv + `=["--","--ozone-platform=x11"]`})
	cases := []struct {
		name   string
		argv   []string
		getenv func(string) string
		want   []string
	}{
		{"its own argv wins", []string{"--", "--use-gl=egl"}, handed, []string{"--", "--use-gl=egl"}},
		{"none of its own takes the handed ones", nil, handed, []string{"--", "--ozone-platform=x11"}},
		{"nothing either way", nil, envReader(nil), nil},
	}
	for _, c := range cases {
		got, err := LaunchArgs(c.argv, c.getenv)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%s: LaunchArgs = %v, %v, want %v", c.name, got, err, c.want)
		}
	}
	if _, err := LaunchArgs(nil, envReader([]string{ArgsEnv + "=--ozone"})); err == nil {
		t.Error("LaunchArgs of a value that is no JSON list = nil error, want one")
	}
}

func TestWithoutMarkerDropsTheHandedArgsToo(t *testing.T) {
	got := WithoutMarker([]string{"PATH=/bin", ArgsEnv + `=["--"]`, WaitEnv + "=1"})
	if !slices.Equal(got, []string{"PATH=/bin"}) {
		t.Fatalf("WithoutMarker = %v, want only PATH", got)
	}
}

// envReader reads one environment the way os.Getenv reads the process's.
func envReader(env []string) func(string) string {
	return func(key string) string {
		for _, kv := range env {
			if k, v, ok := strings.Cut(kv, "="); ok && k == key {
				return v
			}
		}
		return ""
	}
}
