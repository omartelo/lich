package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/restart"
	"github.com/omartelo/lich/internal/system"
)

// nativeEnv pins the native window to launch, by path, the way LICH_SHELL pins
// the web one. `go run` and a native build of one's own have no lich-native
// beside the lich binary.
const nativeEnv = "LICH_NATIVE"

// nativeBinary is the native window's executable, shipped beside lich's.
const nativeBinary = "lich-native"

// nativeStartWait bounds waiting for a lich started here to answer. Most of it
// is the workspace database opening and the login shell resolving.
const nativeStartWait = 15 * time.Second

// native opens the native window on the running lich, starting one without a
// window when none runs, and closes the web window first: both endpoints the
// windows live on take one client at a time, and the newest wins.
func (c *client) native(args []string) error {
	flags := newFlagSet("native")
	if err := c.parse(flags, args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return usageError("native")
	}
	window, err := c.nativeWindow()
	if err != nil {
		return err
	}
	port, token, err := c.backend()
	if err != nil {
		return err
	}
	closed := c.call(context.Background(), "system.CloseWindow", []any{system.CloseWindowOptions{}}, shortCall, nil)
	if closed != nil && !strings.Contains(closed.Error(), restart.ErrNoWindow.Error()) {
		return fmt.Errorf("close the web window: %w", closed)
	}
	env := append(os.Environ(), "LICH_PORT="+port, "LICH_TOKEN="+token)
	if err := c.start(window, env, nil); err != nil {
		return fmt.Errorf("launch %s: %w", window, err)
	}
	fmt.Fprintf(c.stdout, "Opening the native window on lich (port %s).\n", port)
	return nil
}

// nativeWindow is the native window to launch: the pinned one, else the one
// beside this lich binary. A missing one is reported before anything starts.
func (c *client) nativeWindow() (string, error) {
	if pinned := c.env(nativeEnv); pinned != "" {
		if _, err := os.Stat(pinned); err != nil {
			return "", fmt.Errorf("%s=%s: %w", nativeEnv, pinned, err)
		}
		return pinned, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("find the lich binary: %w", err)
	}
	beside := filepath.Join(filepath.Dir(exe), nativeBinary)
	if _, err := os.Stat(beside); err != nil {
		return "", fmt.Errorf("no %s beside %s: build it from native/ (native/slice.sh), "+
			"or point %s at one", nativeBinary, exe, nativeEnv)
	}
	return beside, nil
}

// backend is the running lich's coordinates, starting lich in the background
// with no window when nothing answers on them.
func (c *client) backend() (string, string, error) {
	port, token, err := c.coordinates()
	if err == nil && listening(port) {
		return port, token, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("find the lich binary: %w", err)
	}
	if err := c.start(exe, os.Environ(), []string{"--no-window"}); err != nil {
		return "", "", fmt.Errorf("start lich: %w", err)
	}
	return c.awaitBackend(nativeStartWait)
}

// awaitBackend waits up to wait for a lich just started to write its runtime
// file and listen on the port it names.
func (c *client) awaitBackend(wait time.Duration) (string, string, error) {
	deadline := time.Now().Add(wait)
	for {
		port, token, err := c.coordinates()
		if err == nil && listening(port) {
			return port, token, nil
		}
		if time.Now().After(deadline) {
			return "", "", errors.New("lich was started and did not answer within " + wait.String() +
				"; its log says why (lich doctor walks the boot)")
		}
		time.Sleep(quitPoll)
	}
}

// start launches exe detached, so it outlives this command; nil c.launch takes
// the real one.
func (c *client) start(exe string, env, args []string) error {
	if c.launch != nil {
		return c.launch(exe, env, args)
	}
	return restart.StartDetached(exe, env, args)
}
