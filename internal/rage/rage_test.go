package rage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/omartelo/lich/internal/agentplugin"
	"github.com/omartelo/lich/internal/providers"
	"github.com/omartelo/lich/internal/singleton"
)

// liveToken is the loopback credential of the fake running instance: long
// enough that scrub masks it, and distinctive enough that finding it anywhere
// in the bundle is a failure.
const liveToken = "s3cr3t-loopback-token"

// collector builds a Collector over a temp config dir with every probe
// answered, so no test reaches PATH, a provider CLI or the network.
func collector(t *testing.T, configDir string, info *singleton.Info, alive bool) *Collector {
	t.Helper()
	c := New(configDir, "v1.2.3", []string{"SHELL=/bin/zsh", "LICH_TOKEN=" + liveToken, "LICH_DEV=1"})
	c.browser = func() (string, error) { return "/usr/bin/chromium", nil }
	c.detect = func() []providers.Detected {
		return []providers.Detected{{ID: "claude", Name: "Claude Code", Installed: true, Path: "/usr/bin/claude"}}
	}
	c.plugin = func() []agentplugin.Status {
		return []agentplugin.Status{{Provider: "claude", Name: "Claude Code", Available: true, Installed: true, InstalledVersion: "0.2.0"}}
	}
	c.probe = func() (*singleton.Info, bool) { return info, alive }
	return c
}

// lichDir prepares a config dir holding both log generations, and returns the
// OS-level config dir to hand the collector.
func lichDir(t *testing.T, live, old string) string {
	t.Helper()
	configDir := t.TempDir()
	dir := filepath.Join(configDir, "lich")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// The collector resolves the log through logging.Path, which reads LICH_DEV.
	t.Setenv("LICH_DEV", "")
	if err := os.WriteFile(filepath.Join(dir, "lich.log"), []byte(live), 0o600); err != nil {
		t.Fatal(err)
	}
	if old != "" {
		if err := os.WriteFile(filepath.Join(dir, "lich.log.old"), []byte(old), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return configDir
}

// unpack reads the whole bundle back as name → contents.
func unpack(t *testing.T, data []byte) map[string]string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	out := map[string]string{}
	archive := tar.NewReader(gz)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		body, err := io.ReadAll(archive)
		if err != nil {
			t.Fatalf("read %s: %v", header.Name, err)
		}
		out[header.Name] = string(body)
	}
	return out
}

func bundle(t *testing.T, c *Collector, root string) (map[string]string, Report) {
	t.Helper()
	var buf bytes.Buffer
	report, err := c.Bundle(&buf, root)
	if err != nil {
		t.Fatalf("Bundle: %v", err)
	}
	return unpack(t, buf.Bytes()), report
}

func TestBundleCarriesTheReportTheEnvAndBothLogs(t *testing.T) {
	configDir := lichDir(t, "live line\n", "older line\n")
	c := collector(t, configDir, &singleton.Info{PID: 42, Port: 47821, Token: liveToken}, true)

	files, report := bundle(t, c, "lich-rage-x")

	for _, want := range []string{
		"lich-rage-x/report.md",
		"lich-rage-x/env.txt",
		"lich-rage-x/logs/lich.log",
		"lich-rage-x/logs/lich.log.old",
	} {
		if _, ok := files[want]; !ok {
			t.Errorf("bundle has no %s (got %v)", want, keys(files))
		}
	}
	if !strings.Contains(files["lich-rage-x/logs/lich.log"], "live line") {
		t.Error("the live log generation is not the one that was copied")
	}
	if report.Version != "v1.2.3" || !strings.Contains(report.Instance, "pid 42") {
		t.Errorf("report = %+v", report)
	}
}

func TestBundleNeverCarriesTheLoopbackToken(t *testing.T) {
	configDir := lichDir(t,
		"time=x msg=\"chromium shell opening\" url=http://127.0.0.1:47821/?token="+liveToken+" more=1\n"+
			"time=y msg=stale url=http://127.0.0.1:47821/?token=an-older-instances-token\n", "")
	c := collector(t, configDir, &singleton.Info{PID: 42, Port: 47821, Token: liveToken}, true)

	files, _ := bundle(t, c, "r")

	for name, body := range files {
		if strings.Contains(body, liveToken) {
			t.Errorf("%s leaks the live token", name)
		}
		if strings.Contains(body, "an-older-instances-token") {
			t.Errorf("%s leaks an earlier instance's token", name)
		}
	}
	log := files["r/logs/lich.log"]
	if !strings.Contains(log, "more=1") {
		t.Errorf("masking ate what followed the token: %q", log)
	}
	if !strings.Contains(files["r/env.txt"], "LICH_TOKEN=<SET>") {
		t.Errorf("env.txt does not report the token as present: %q", files["r/env.txt"])
	}
}

func TestBundleCarriesTheTailOfAnOversizedLog(t *testing.T) {
	head := strings.Repeat("a", maxLogBytes)
	configDir := lichDir(t, head+"the end\n", "")
	c := collector(t, configDir, nil, false)

	files, _ := bundle(t, c, "r")

	log := files["r/logs/lich.log"]
	if !strings.Contains(log, "the end") {
		t.Error("the tail — the half that holds what just went wrong — was dropped")
	}
	if !strings.Contains(log, "earlier bytes omitted") {
		t.Errorf("a truncated log does not say so: %q", log[:80])
	}
	if len(log) > maxLogBytes+200 {
		t.Errorf("log entry is %d bytes, want the cap plus its marker", len(log))
	}
}

func TestReportNamesEveryInstanceState(t *testing.T) {
	cases := []struct {
		name  string
		info  *singleton.Info
		alive bool
		want  string
	}{
		{"none recorded", nil, false, "not running"},
		{"answering", &singleton.Info{PID: 7, Port: 47821, Token: liveToken}, true, "running (pid 7, port 47821)"},
		{"stale file", &singleton.Info{PID: 7, Port: 47821, Token: liveToken}, false, "recorded but not answering (pid 7, port 47821)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := collector(t, lichDir(t, "x\n", ""), tc.info, tc.alive)
			_, report := bundle(t, c, "r")
			if report.Instance != tc.want {
				t.Errorf("instance = %q, want %q", report.Instance, tc.want)
			}
		})
	}
}

func TestBundleSurvivesAConfigDirWithNoLog(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("LICH_DEV", "")
	c := collector(t, configDir, nil, false)

	files, report := bundle(t, c, "r")

	if report.LogPath != "" {
		t.Errorf("log path = %q, want empty when no file was ever written", report.LogPath)
	}
	for name := range files {
		if strings.HasPrefix(name, "r/logs/") {
			t.Errorf("bundle carries %s with no log on disk", name)
		}
	}
	if !strings.Contains(files["r/report.md"], "stderr only") {
		t.Error("report.md does not say the log is missing")
	}
}

func TestReportListsTheConfigDirectoryWithoutDescending(t *testing.T) {
	configDir := lichDir(t, "x\n", "")
	dir := filepath.Join(configDir, "lich")
	if err := os.MkdirAll(filepath.Join(dir, "chromium-profile", "Default"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chromium-profile", "Default", "Cookies"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lich.db"), []byte("sqlite"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := collector(t, configDir, nil, false)

	files, report := bundle(t, c, "r")

	var names []string
	for _, e := range report.Entries {
		names = append(names, e.Name)
	}
	for _, want := range []string{"lich.db", "chromium-profile"} {
		if !contains(names, want) {
			t.Errorf("entries %v do not list %s", names, want)
		}
	}
	if contains(names, "Default") || contains(names, "Cookies") {
		t.Errorf("entries descended into a directory: %v", names)
	}
	if strings.Contains(files["r/report.md"], "Cookies") {
		t.Error("report.md names a file inside the Chromium profile")
	}
}

func TestDefaultBaseNamesTheMoment(t *testing.T) {
	got := DefaultBase(time.Date(2026, 8, 10, 15, 4, 5, 0, time.UTC))
	if got != "lich-rage-20260810-150405" {
		t.Errorf("DefaultBase = %q", got)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestBundleReportsAWriteFailure pins that a broken destination is an error the
// caller can act on, not a silent half-archive.
func TestBundleReportsAWriteFailure(t *testing.T) {
	c := collector(t, lichDir(t, "x\n", ""), nil, false)
	if _, err := c.Bundle(failingWriter{}, "r"); err == nil {
		t.Fatal("Bundle succeeded against a writer that fails every write")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("disk full") }
