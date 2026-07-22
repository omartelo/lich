package restart

import (
	"errors"
	"os"
	"slices"
	"testing"
)

func TestNew(t *testing.T) {
	c := New("/usr/local/bin/lich", []string{"PATH=/bin"})
	if c.exePath != "/usr/local/bin/lich" {
		t.Fatalf("exePath = %q", c.exePath)
	}
	if c.spawn == nil || c.terminate == nil {
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

func TestDoSpawnsThenTerminates(t *testing.T) {
	var (
		spawnedEnv []string
		terminated bool
		order      []string
	)
	c := &Coordinator{
		exePath: "/usr/local/bin/lich",
		env:     []string{"PATH=/bin"},
		spawn: func(_ string, env []string) error {
			spawnedEnv = env
			order = append(order, "spawn")
			return nil
		},
		terminate: func(*os.Process) error {
			terminated = true
			order = append(order, "terminate")
			return nil
		},
	}
	c.SetWindow(&os.Process{}) // non-nil so terminate runs

	if err := c.Do(); err != nil {
		t.Fatalf("Do() = %v, want nil", err)
	}
	if !terminated {
		t.Fatal("window was not terminated")
	}
	if !slices.Contains(spawnedEnv, WaitEnv+"=1") {
		t.Fatalf("successor env = %v, missing wait marker", spawnedEnv)
	}
	// Successor must start before the window is torn down.
	if len(order) != 2 || order[0] != "spawn" || order[1] != "terminate" {
		t.Fatalf("order = %v, want [spawn terminate]", order)
	}
}

func TestDoIsIdempotent(t *testing.T) {
	spawns, terminates := 0, 0
	c := &Coordinator{
		exePath:   "/usr/local/bin/lich",
		spawn:     func(string, []string) error { spawns++; return nil },
		terminate: func(*os.Process) error { terminates++; return nil },
	}
	c.SetWindow(&os.Process{})

	for range 3 {
		if err := c.Do(); err != nil {
			t.Fatalf("Do() = %v, want nil", err)
		}
	}
	if spawns != 1 || terminates != 1 {
		t.Fatalf("spawns=%d terminates=%d, want 1 and 1 — restart must fire once", spawns, terminates)
	}
}

func TestDoWithoutWindowStillSpawns(t *testing.T) {
	spawned := false
	c := &Coordinator{
		exePath:   "/usr/local/bin/lich",
		spawn:     func(string, []string) error { spawned = true; return nil },
		terminate: func(*os.Process) error { t.Fatal("terminate called with no window"); return nil },
	}
	if err := c.Do(); err != nil {
		t.Fatalf("Do() = %v, want nil", err)
	}
	if !spawned {
		t.Fatal("successor was not spawned")
	}
}

func TestDoErrors(t *testing.T) {
	t.Run("no exe path", func(t *testing.T) {
		c := &Coordinator{spawn: func(string, []string) error { return nil }}
		if err := c.Do(); err == nil {
			t.Fatal("Do() = nil, want error when exe path is unknown")
		}
	})

	t.Run("spawn failure propagates", func(t *testing.T) {
		c := &Coordinator{
			exePath: "/usr/local/bin/lich",
			spawn:   func(string, []string) error { return errors.New("boom") },
		}
		if err := c.Do(); err == nil {
			t.Fatal("Do() = nil, want spawn error")
		}
	})
}
