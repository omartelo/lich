// Package doctor answers "would lich start on this machine, and why not". It
// walks the same boot the app walks — config dir, log file, the pinned listener,
// the workspace database, the browser, the providers — and reports each step
// with a verdict and how long it took.
//
// It is a command line because the failure it diagnoses is a window that never
// opened: there is no Settings › Help behind a window that is not there. It
// needs no TTY and no running instance, and it exits non-zero when a check that
// would stop a launch fails, so it works as a smoke test in a script.
package doctor

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/omartelo/lich/internal/logging"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/singleton"
	"github.com/omartelo/lich/internal/store"
)

// Status is one check's verdict.
type Status string

const (
	// OK: this step of a launch would succeed.
	OK Status = "ok"
	// Warn: lich starts, but something it offers will not work.
	Warn Status = "warn"
	// Fail: a launch stops here. Any Fail exits non-zero.
	Fail Status = "fail"
	// Skip: the check could not run without disturbing the instance already
	// running — reported rather than guessed at.
	Skip Status = "skip"
)

// Check is one step of the boot, as the report prints it.
type Check struct {
	Name   string
	Status Status
	Detail string
	Took   time.Duration
}

// bindProbe holds the pinned port for as long as it takes to prove it is free.
// Nothing is served on it: a launch seconds later has to be able to take it.
func bindProbe(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	return listener.Close()
}

// Doctor runs the checks. Every probe that reaches the machine is a field, so a
// test decides what this machine has without touching PATH, the port or the
// database.
type Doctor struct {
	configDir string
	port      int

	bind     func(port int) error
	instance func() (*singleton.Info, bool)
	browser  func() (string, error)
	detect   func() []providers.Detected
	store    func() (io.Closer, error)
}

// New returns a Doctor for this machine. configDir is the OS config dir, and
// getenv resolves the listener port exactly as a launch would.
func New(configDir string, getenv func(string) string) *Doctor {
	return &Doctor{
		configDir: configDir,
		port:      singleton.Port(getenv),
		bind:      bindProbe,
		instance:  func() (*singleton.Info, bool) { return Instance(configDir) },
		browser:   Browser,
		detect:    Providers,
		store:     func() (io.Closer, error) { return store.New() },
	}
}

// Run walks the boot in order and returns one Check per step. The instance is
// probed once for the whole run: two checks need the answer, and a stale
// runtime file costs a timeout to find out.
func (d *Doctor) Run() []Check {
	info, alive := d.instance()
	running := alive && info != nil

	steps := []struct {
		name string
		run  func() (Status, string)
	}{
		{"home", d.checkHome},
		{"log", d.checkLog},
		{"listener", func() (Status, string) { return d.checkListener(info, running) }},
		{"store", func() (Status, string) { return d.checkStore(info, running) }},
		{"browser", d.checkBrowser},
		{"providers", d.checkProviders},
	}

	checks := make([]Check, 0, len(steps))
	for _, step := range steps {
		started := time.Now()
		status, detail := step.run()
		checks = append(checks, Check{
			Name: step.name, Status: status, Detail: detail, Took: time.Since(started),
		})
	}
	return checks
}

// checkHome proves lich's config directory can be created and written to. It is
// first because every step below it writes there.
func (d *Doctor) checkHome() (Status, string) {
	dir := filepath.Join(d.configDir, "lich")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Fail, fmt.Sprintf("%s cannot be created: %v", dir, err)
	}
	probe := filepath.Join(dir, ".doctor-probe")
	if err := os.WriteFile(probe, []byte("lich doctor"), 0o600); err != nil {
		return Fail, fmt.Sprintf("%s is not writable: %v", dir, err)
	}
	if err := os.Remove(probe); err != nil {
		return Warn, fmt.Sprintf("%s is writable, but %s could not be removed", dir, probe)
	}
	return OK, dir + " is writable"
}

// checkLog opens the log the way startup does, minus the rotation: rotating
// here would move the generation a bug report is about to attach.
func (d *Doctor) checkLog() (Status, string) {
	path := logging.Path(filepath.Join(d.configDir, "lich"))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return Warn, fmt.Sprintf("no log file — lich would run on stderr only: %v", err)
	}
	if err := file.Close(); err != nil {
		return Warn, fmt.Sprintf("%s does not close cleanly: %v", path, err)
	}
	return OK, path
}

// checkListener asks the question a failed launch poses: is the pinned port
// free, held by the lich already running (which is the single-instance lock
// working), or taken by something else that will not hand it over.
func (d *Doctor) checkListener(info *singleton.Info, running bool) (Status, string) {
	err := d.bind(d.port)
	if err == nil {
		return OK, fmt.Sprintf("port %d is free", d.port)
	}
	if running && info.Port == d.port {
		return OK, fmt.Sprintf("port %d is held by the running lich (pid %d)", d.port, info.PID)
	}
	// Not "taken": the same bind refuses a privileged port the same way, and a
	// diagnosis that names the wrong cause sends the reader after the wrong fix.
	return Fail, fmt.Sprintf("port %d cannot be bound: %v", d.port, err)
}

// checkStore opens and migrates the workspace database, which is what a launch
// does — but never while an instance is running: a second process migrating the
// store is not a price a diagnosis may charge.
func (d *Doctor) checkStore(info *singleton.Info, running bool) (Status, string) {
	if running {
		return Skip, fmt.Sprintf("held by the running lich (pid %d)", info.PID)
	}
	db, err := d.store()
	if err != nil {
		return Fail, fmt.Sprintf("the workspace database will not open: %v", err)
	}
	if err := db.Close(); err != nil {
		return Warn, fmt.Sprintf("the workspace database does not close cleanly: %v", err)
	}
	return OK, "the workspace database opens and its schema is current"
}

// checkBrowser resolves the Chromium-family binary the window is. Its absence
// is the one failure where lich runs perfectly and shows nothing.
func (d *Doctor) checkBrowser() (Status, string) {
	path, err := d.browser()
	if err != nil {
		return Fail, err.Error()
	}
	return OK, path
}

// checkProviders counts the harnesses a session could spawn. None is not a
// launch failure — the window opens on an empty workspace — so it warns.
func (d *Doctor) checkProviders() (Status, string) {
	found := d.detect()
	var installed []string
	for _, p := range found {
		if p.Installed {
			installed = append(installed, p.ID)
		}
	}
	if len(installed) == 0 {
		return Warn, "none on PATH — lich opens, but no session can spawn"
	}
	return OK, fmt.Sprintf("%d of %d on PATH: %s",
		len(installed), len(found), strings.Join(installed, ", "))
}

// Failed reports whether any check would stop a launch, which is what the exit
// code carries to a script.
func Failed(checks []Check) bool {
	for _, c := range checks {
		if c.Status == Fail {
			return true
		}
	}
	return false
}
