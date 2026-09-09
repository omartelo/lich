//go:build windows && installer_e2e

package appupdate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Runs the real backend and production Inno script. Only Chromium is a fixture:
// a WinForms window with a child holding libcef.dll until a delayed clean close.
func TestWindowsInstallerE2E(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	install := filepath.Join(work, "installed lich")
	build := filepath.Join(work, "build", "windows")
	bin := filepath.Join(work, "bin")
	for _, dir := range []string{install, build, filepath.Join(bin, "shell")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"lich.iss", "lich.ico"} {
		copyFixture(t, filepath.Join(root, "build", "windows", name), filepath.Join(build, name))
	}
	csc := filepath.Join(os.Getenv("WINDIR"), "Microsoft.NET", "Framework64", "v4.0.30319", "csc.exe")
	runFixture(t, root, csc, "/nologo", "/target:winexe", "/r:System.Windows.Forms.dll",
		"/out:"+filepath.Join(bin, "shell", "lich-shell.exe"), filepath.Join(root, "internal", "appupdate", "testdata", "window.cs"))
	asset := "lich-v0.8.0-windows-amd64-setup.exe"
	installer := filepath.Join(bin, "lich-setup.exe")
	srv := installerReleaseFixture(t, installer, asset)
	defer srv.Close()
	overlay := releaseOverlay(t, root, work, srv.URL)
	oldExe := filepath.Join(install, "lich.exe")
	runFixture(t, root, "go", "build", "-overlay", overlay, "-ldflags=-H=windowsgui -X main.version=0.7.0", "-o", oldExe, ".")
	runFixture(t, root, "go", "build", "-overlay", overlay, "-ldflags=-H=windowsgui -X main.version=0.8.0", "-o", filepath.Join(bin, "lich.exe"), ".")
	if err := os.CopyFS(filepath.Join(install, "shell"), os.DirFS(filepath.Join(bin, "shell"))); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(install, "shell", "libcef.dll"), []byte("0.7.0"))
	writeFixture(t, filepath.Join(bin, "shell", "libcef.dll"), []byte("0.8.0"))
	t.Setenv("VERSION", "0.8.0")
	iscc := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Inno Setup 6", "ISCC.exe")
	runFixture(t, root, iscc, "/Qp", filepath.Join(build, "lich.iss"))
	runInstalledUpdate(t, install, bin, work)
}

func installerReleaseFixture(t *testing.T, installer, asset string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			fmt.Fprint(w, `{"tag_name":"v0.8.0"}`)
		case "/v0.8.0/checksums.txt":
			data, err := os.ReadFile(installer)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			fmt.Fprintf(w, "%x  %s\n", sha256.Sum256(data), asset)
		case "/v0.8.0/" + asset:
			http.ServeFile(w, r, installer)
		default:
			http.NotFound(w, r)
		}
	}))
}

// An overlay changes only the release URLs in the test builds. Shipped binaries
// have no environment switch allowing a local process to redirect their updates.
func releaseOverlay(t *testing.T, root, work, url string) string {
	t.Helper()
	source := filepath.Join(root, "internal", "appupdate", "appupdate.go")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), `"https://api.github.com/repos/" + repo + "/releases/latest"`, fmt.Sprintf("%q", url+"/latest"), 1)
	text = strings.Replace(text, `"https://github.com/" + repo + "/releases/download/"`, fmt.Sprintf("%q", url+"/"), 1)
	patched := filepath.Join(work, "appupdate.go")
	writeFixture(t, patched, []byte(text))
	data, err = json.Marshal(map[string]any{"Replace": map[string]string{source: patched}})
	if err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(work, "overlay.json")
	writeFixture(t, overlay, data)
	return overlay
}

func runInstalledUpdate(t *testing.T, install, bin, work string) {
	t.Helper()
	t.Setenv("APPDATA", filepath.Join(work, "config"))
	t.Setenv("LICH_LISTEN_PORT", "47983")
	t.Setenv("LICH_UPDATE_EVENTS", filepath.Join(work, "events.txt"))
	t.Setenv("LICH_BROWSER", "")
	t.Setenv("LICH_NO_WINDOW", "")
	t.Setenv("CGO_ENABLED", "0")
	cmd := exec.Command(filepath.Join(install, "lich.exe"))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupInstalledFixture(t, install, work, cmd.Process.Pid) })
	waitFixture(t, func() bool {
		return fixtureVersion(work) == "0.7.0" && strings.Contains(fixtureEvents(work), "window ready 0.7.0")
	})
	oldToken := fixtureToken(work)
	resp, err := http.Post("http://127.0.0.1:47983/rpc/appupdate.Apply?token="+oldToken, "application/json", strings.NewReader("[]"))
	// Closing the window may end the RPC connection before its response flushes.
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Apply status = %d", resp.StatusCode)
		}
	}
	waitFixture(t, func() bool {
		return fixtureVersion(work) == "0.8.0" && strings.Contains(fixtureEvents(work), "window ready 0.8.0")
	})
	if err := cmd.Wait(); err != nil {
		t.Fatalf("old lich did not exit cleanly: %v", err)
	}
	if fixtureToken(work) == oldToken {
		t.Fatal("successor did not take the port")
	}
	assertInstalledUpdate(t, install, bin, work)
}

func cleanupInstalledFixture(t *testing.T, install, work string, pid int) {
	t.Helper()
	if t.Failed() {
		for _, path := range []string{filepath.Join(install, ".lich-update-setup.log"), filepath.Join(work, "config", "lich", "lich.log"), filepath.Join(work, "events.txt")} {
			data, err := os.ReadFile(path)
			t.Logf("%s: %v\n%s", path, err, data)
		}
	}
	// Only this fixture's process tree; never match an installed lich by name.
	if data, err := os.ReadFile(filepath.Join(work, "config", "lich", "runtime.json")); err == nil {
		var runtime struct {
			PID int `json:"pid"`
		}
		if json.Unmarshal(data, &runtime) == nil && runtime.PID > 0 {
			_ = exec.Command("taskkill.exe", "/F", "/T", "/PID", fmt.Sprint(runtime.PID)).Run()
		}
	}
	_ = exec.Command("taskkill.exe", "/F", "/T", "/PID", fmt.Sprint(pid)).Run()
	_ = exec.Command(filepath.Join(install, "unins000.exe"), "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART").Run()
}

func assertInstalledUpdate(t *testing.T, install, bin, work string) {
	t.Helper()
	installed, err := os.ReadFile(filepath.Join(install, "lich.exe"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(bin, "lich.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(installed) != sha256.Sum256(want) {
		t.Fatal("installed exe was not replaced")
	}
	events := fixtureEvents(work)
	last := -1
	for _, event := range []string{"window ready 0.7.0", "close requested 0.7.0", "locks released", "window closed 0.7.0", "window ready 0.8.0"} {
		index := strings.Index(events, event)
		if index <= last {
			t.Fatalf("incorrect shutdown order for %q:\n%s", event, events)
		}
		last = index
	}
	log, err := os.ReadFile(filepath.Join(install, ".lich-update-setup.log"))
	if err != nil {
		t.Fatal(err)
	}
	// Inno's log may be UTF-16; strip NULs for these ASCII assertions.
	text := strings.ReplaceAll(string(log), "\x00", "")
	if !strings.Contains(text, "outgoing process exited before file replacement") {
		t.Fatalf("installer did not await lich:\n%s", text)
	}
	t.Logf("installer replaced exe and locked DLL; successor owns port 47983\n%s", events)
}

func fixtureToken(work string) string {
	data, err := os.ReadFile(filepath.Join(work, "config", "lich", "runtime.json"))
	if err != nil {
		return ""
	}
	var runtime struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(data, &runtime) != nil {
		return ""
	}
	return runtime.Token
}

func fixtureVersion(work string) string {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Post("http://127.0.0.1:47983/rpc/system.Diagnostics?token="+fixtureToken(work), "application/json", strings.NewReader("[]"))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var info struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(resp.Body).Decode(&info) != nil {
		return ""
	}
	return info.Version
}

func fixtureEvents(work string) string {
	data, _ := os.ReadFile(filepath.Join(work, "events.txt"))
	return string(data)
}

func waitFixture(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timed out waiting for installed lich")
}

func runFixture(t *testing.T, dir, exe string, args ...string) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", exe, err, out)
	}
}

func copyFixture(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, to, data)
}

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
