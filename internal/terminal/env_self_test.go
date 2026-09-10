//go:build darwin || windows

package terminal

import (
	"os"
	"os/exec"
	"testing"
)

// The only readEnv this suite can point at a process it fully controls is its
// own: both platforms read a live process, so the fixture has to be a process,
// and the child re-runs this same test to be one.
func TestProcessEnvironmentReadsSelf(t *testing.T) {
	const marker = "LICH_TEST_PROCESS_ENV"
	const value = "café😀=account"
	if os.Getenv(marker) == value {
		env := readEnv(os.Getpid())
		if env == nil || env[marker] != value {
			t.Fatal("could not read the test process's own environment")
		}
		if empty, defined := env["LICH_TEST_EMPTY_ENV"]; !defined || empty != "" {
			t.Fatal("defined empty value was lost")
		}
		return
	}
	// Darwin reports the exec-time environment, so set the fixture before exec.
	cmd := exec.Command(os.Args[0], "-test.run=^TestProcessEnvironmentReadsSelf$")
	cmd.Env = append(os.Environ(), marker+"="+value, "LICH_TEST_EMPTY_ENV=")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child environment check: %v\n%s", err, output)
	}
	for _, pid := range []int{-1, 0, 1 << 30} {
		if readEnv(pid) != nil {
			t.Errorf("invalid pid %d must be unknown", pid)
		}
	}
}
