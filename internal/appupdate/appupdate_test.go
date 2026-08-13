package appupdate

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch string
		want         string
	}{
		{"windows", "amd64", "lich-v0.8.0-windows-amd64.exe"},
		{"darwin", "arm64", "lich-v0.8.0-darwin-arm64"},
		{"darwin", "amd64", "lich-v0.8.0-darwin-amd64"},
		{"linux", "amd64", ""},   // package-manager owned, no self-apply asset
		{"windows", "arm64", ""}, // no arm64 windows asset ships
		{"darwin", "386", ""},
	}
	for _, tc := range tests {
		if got := assetName(tc.goos, tc.goarch, "0.8.0"); got != tc.want {
			t.Errorf("assetName(%q,%q) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}

func TestParseChecksum(t *testing.T) {
	data := []byte(
		"aaaa1111  lich-v0.8.0-linux-amd64\n" +
			"bbbb2222  lich-v0.8.0-darwin-arm64\n" +
			"cccc3333 *lich-v0.8.0-windows-amd64.exe\n", // binary-mode marker
	)
	tests := []struct {
		asset, want string
	}{
		{"lich-v0.8.0-darwin-arm64", "bbbb2222"},
		{"lich-v0.8.0-windows-amd64.exe", "cccc3333"},
		{"lich-v0.8.0-linux-amd64", "aaaa1111"},
		{"lich-v9.9.9-nope", ""},
	}
	for _, tc := range tests {
		if got := parseChecksum(data, tc.asset); got != tc.want {
			t.Errorf("parseChecksum(%q) = %q, want %q", tc.asset, got, tc.want)
		}
	}
}

func TestCanSelfApply(t *testing.T) {
	writable := t.TempDir()
	exe := filepath.Join(writable, "lich")

	tests := []struct {
		name    string
		goos    string
		exePath string
		want    bool
	}{
		{"windows writable dir", "windows", exe, true},
		{"darwin writable dir", "darwin", exe, true},
		{"linux never", "linux", exe, false},
		{"no exe path", "darwin", "", false},
		{"unwritable dir", "darwin", filepath.Join("/nonexistent-abc123", "lich"), false},
		{"homebrew cellar is brew's", "darwin", cellarExe(t), false},
	}
	for _, tc := range tests {
		if got := canSelfApply(tc.goos, tc.exePath); got != tc.want {
			t.Errorf("canSelfApply(%q,%q) = %v, want %v", tc.goos, tc.exePath, got, tc.want)
		}
	}
}

// cellarExe returns a writable path shaped like a Homebrew formula install
// (<prefix>/Cellar/<formula>/<version>/bin/lich), so the Cellar segment is the
// only reason canSelfApply can refuse it.
func cellarExe(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "Cellar", "lich", "0.21.1", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "lich")
}

// The binary users actually launch is <prefix>/bin/lich, a symlink into the
// Cellar — the check has to see through it.
func TestBrewOwnedResolvesLinkedBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on Windows")
	}
	cellar := cellarExe(t)
	if err := os.WriteFile(cellar, []byte("lich"), 0o755); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(linkDir, "lich")
	if err := os.Symlink(cellar, link); err != nil {
		t.Fatal(err)
	}
	if !brewOwned(link) {
		t.Errorf("brewOwned(%q) = false, want true through the symlink", link)
	}
	if canSelfApply("darwin", link) {
		t.Error("canSelfApply = true for a brew-linked binary, want false")
	}
}

// serveBody points a Service's latest endpoint at a test server.
func serveBody(t *testing.T, status int, body string) *Service {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return &Service{
		http:      srv.Client(),
		goos:      runtime.GOOS,
		latestURL: srv.URL,
		tagBase:   releaseTagBase,
	}
}

func TestStatus(t *testing.T) {
	// A writable exe dir so CanSelfApply is deterministic on the darwin/windows
	// branch; the test forces the version comparison, not the platform.
	s := serveBody(t, http.StatusOK, `{"tag_name":"v0.8.0"}`)
	s.exePath = filepath.Join(t.TempDir(), "lich")

	t.Run("update available", func(t *testing.T) {
		s.version = "0.7.0"
		got := s.Status()
		if !got.UpdateAvailable {
			t.Fatalf("UpdateAvailable = false, want true (%+v)", got)
		}
		if got.CurrentVersion != "0.7.0" || got.LatestVersion != "0.8.0" {
			t.Fatalf("versions = %+v", got)
		}
		if got.ReleaseURL != "https://github.com/omartelo/lich/releases/tag/v0.8.0" {
			t.Fatalf("ReleaseURL = %q", got.ReleaseURL)
		}
		// CanSelfApply tracks the platform: only the self-apply OSes on a
		// writable dir. Linux (the CI host) must report false.
		wantSelfApply := runtime.GOOS == "windows" || runtime.GOOS == "darwin"
		if got.CanSelfApply != wantSelfApply {
			t.Fatalf("CanSelfApply = %v, want %v on %s", got.CanSelfApply, wantSelfApply, runtime.GOOS)
		}
	})

	t.Run("already latest", func(t *testing.T) {
		s.version = "0.8.0"
		if s.Status().UpdateAvailable {
			t.Fatal("UpdateAvailable = true, want false when on the latest")
		}
	})

	t.Run("dev build never updates", func(t *testing.T) {
		s.version = "dev"
		got := s.Status()
		if got.UpdateAvailable {
			t.Fatal("UpdateAvailable = true, want false for a dev build")
		}
		if got.CurrentVersion != "dev" {
			t.Fatalf("CurrentVersion = %q, want dev", got.CurrentVersion)
		}
	})

	// The lookup runs at startup: a GitHub that answers 500 must leave the
	// readout empty rather than claim an update or block the boot.
	t.Run("a failed lookup reports nothing", func(t *testing.T) {
		failing := serveBody(t, http.StatusInternalServerError, "upstream is down")
		failing.version = "0.7.0"

		got := failing.Status()

		if got.UpdateAvailable {
			t.Error("UpdateAvailable = true, want false when the lookup failed")
		}
		if got.LatestVersion != "" {
			t.Errorf("LatestVersion = %q, want empty", got.LatestVersion)
		}
		if got.ReleaseURL != "" {
			t.Errorf("ReleaseURL = %q, want empty — there is no release to point at", got.ReleaseURL)
		}
		if got.CurrentVersion != "0.7.0" {
			t.Errorf("CurrentVersion = %q, want the running build", got.CurrentVersion)
		}
	})
}

func TestInstallCommand(t *testing.T) {
	arch := "yay -S lich-bin" + restartChain
	brew := "brew upgrade omartelo/tap/lich" + restartChain

	tests := []struct {
		name      string
		goos      string
		exePath   string
		osRelease string // written to a temp file; "" leaves osReleasePath missing
		want      string
	}{
		{"windows self-apply", "windows", "", "", ""},
		{"darwin self-apply", "darwin", "", "", ""},
		{"homebrew install", "darwin", cellarExe(t), "", brew},
		{"arch by ID", "linux", "", "ID=arch\n", arch},
		{"arch quoted ID", "linux", "", "ID=\"arch\"\n", arch},
		{"arch derivative via ID_LIKE", "linux", "", "ID=manjaro\nID_LIKE=arch\n", arch},
		{"debian uses install.sh", "linux", "", "ID=debian\n", installScript},
		{"fedora uses install.sh", "linux", "", "ID=fedora\nID_LIKE=\"rhel centos\"\n", installScript},
		{"missing os-release uses install.sh", "linux", "", "", installScript},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{goos: tc.goos, exePath: tc.exePath, osReleasePath: filepath.Join("/nonexistent-abc123", "os-release")}
			if tc.osRelease != "" {
				path := filepath.Join(t.TempDir(), "os-release")
				if err := os.WriteFile(path, []byte(tc.osRelease), 0o600); err != nil {
					t.Fatal(err)
				}
				s.osReleasePath = path
			}
			if got := s.installCommand(); got != tc.want {
				t.Errorf("installCommand() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestApplyRejectedWhenNotSelfApply(t *testing.T) {
	s := New("0.7.0")
	s.exePath = "" // forces canSelfApply false regardless of platform
	if err := s.Apply(); err == nil {
		t.Fatal("Apply() = nil, want an error when self-apply is unsupported")
	}
}

// applyAsset is the release asset the apply tests drive: the goos/goarch pair
// is pinned on the Service, so the test runs the same on every host.
const applyAsset = "lich-v0.8.0-darwin-arm64"

// applyServer serves the release endpoints Apply hits: the latest-tag JSON,
// checksums.txt (with a matching hash for applyAsset), and the asset bytes
// under assetStatus — 404 is the window between a pushed tag and a published
// binary.
func applyServer(t *testing.T, assetStatus int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			_, _ = io.WriteString(w, `{"tag_name":"v0.8.0"}`)
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			_, _ = io.WriteString(w, "deadbeef  "+applyAsset+"\n")
		default:
			w.WriteHeader(assetStatus)
			_, _ = io.WriteString(w, "FAKEBINARY")
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// applyService is a Service pinned to darwin/arm64 — the self-apply path driven
// off whatever host runs the suite.
func applyService(t *testing.T, srv *httptest.Server, apply func(io.Reader, []byte) error) *Service {
	t.Helper()
	return &Service{
		http:         srv.Client(),
		goos:         "darwin",
		goarch:       "arm64",
		exePath:      filepath.Join(t.TempDir(), "lich"),
		latestURL:    srv.URL + "/latest",
		downloadBase: srv.URL + "/dl/",
		applyBinary:  apply,
	}
}

func TestApplyDownloadsVerifiesAndSwaps(t *testing.T) {
	srv := applyServer(t, http.StatusOK)

	var gotBody string
	var gotChecksum []byte
	s := applyService(t, srv, func(r io.Reader, checksum []byte) error {
		b, _ := io.ReadAll(r)
		gotBody = string(b)
		gotChecksum = checksum
		return nil
	})

	if err := s.Apply(); err != nil {
		t.Fatalf("Apply() = %v, want nil", err)
	}
	if gotBody != "FAKEBINARY" {
		t.Fatalf("swapped body = %q, want the downloaded asset", gotBody)
	}
	// The checksum handed to the swap is the decoded hex from checksums.txt.
	want := []byte{0xde, 0xad, 0xbe, 0xef}
	if len(gotChecksum) != len(want) || gotChecksum[0] != 0xde || gotChecksum[3] != 0xef {
		t.Fatalf("checksum = %x, want %x", gotChecksum, want)
	}
}

func TestApplyPropagatesSwapError(t *testing.T) {
	srv := applyServer(t, http.StatusOK)
	s := applyService(t, srv, func(io.Reader, []byte) error { return io.ErrUnexpectedEOF })
	if err := s.Apply(); err == nil {
		t.Fatal("Apply() = nil, want the swap error propagated")
	}
}

// A tag is pushed before the release job finishes uploading, so the asset can
// be missing while the version lookup already reports the new release.
func TestApplyFailsWhenTheAssetIsNotPublishedYet(t *testing.T) {
	srv := applyServer(t, http.StatusNotFound)
	swapped := false
	s := applyService(t, srv, func(io.Reader, []byte) error {
		swapped = true
		return nil
	})

	err := s.Apply()

	if err == nil {
		t.Fatal("Apply() = nil, want an error when the asset download 404s")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want the status in it", err)
	}
	if swapped {
		t.Error("Apply swapped the binary for a 404 body")
	}
}

// No asset ships for every self-apply platform: a darwin/386 build has nothing
// to download, and saying so beats fetching a URL that cannot exist.
func TestApplyRefusesAnArchWithNoAsset(t *testing.T) {
	srv := applyServer(t, http.StatusOK)
	s := applyService(t, srv, func(io.Reader, []byte) error { return nil })
	s.goarch = "386"

	err := s.Apply()

	if err == nil {
		t.Fatal("Apply() = nil, want an error when no asset ships for the arch")
	}
	if !strings.Contains(err.Error(), "darwin/386") {
		t.Errorf("error = %q, want it to name the platform", err)
	}
}

func TestFetchChecksum(t *testing.T) {
	t.Run("reads the asset's hash", func(t *testing.T) {
		s := serveBody(t, http.StatusOK, "aaaa  lich-v0.8.0-darwin-arm64\nbbbb  other\n")

		sum, err := s.fetchChecksum(s.latestURL, "lich-v0.8.0-darwin-arm64")
		if err != nil {
			t.Fatalf("fetchChecksum() error: %v", err)
		}
		if len(sum) != 2 || sum[0] != 0xaa || sum[1] != 0xaa {
			t.Fatalf("sum = %x, want aaaa", sum)
		}

		if _, err := s.fetchChecksum(s.latestURL, "missing-asset"); err == nil {
			t.Fatal("fetchChecksum(missing) = nil error, want failure")
		}
	})

	t.Run("a non-200 is not a checksums file", func(t *testing.T) {
		s := serveBody(t, http.StatusNotFound, "Not Found")

		_, err := s.fetchChecksum(s.latestURL, "lich-v0.8.0-darwin-arm64")

		if err == nil {
			t.Fatal("fetchChecksum() = nil error, want failure on a non-200")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("error = %q, want the status in it", err)
		}
	})

	// checksums.txt is remote data: a truncated or corrupted line must fail the
	// update, never reach selfupdate as a half-decoded checksum.
	t.Run("a malformed hash is refused", func(t *testing.T) {
		s := serveBody(t, http.StatusOK, "zzzz  lich-v0.8.0-darwin-arm64\n")

		_, err := s.fetchChecksum(s.latestURL, "lich-v0.8.0-darwin-arm64")

		if err == nil {
			t.Fatal("fetchChecksum() = nil error, want failure on a non-hex hash")
		}
		if !strings.Contains(err.Error(), "checksums") {
			t.Errorf("error = %q, want the checksums context on it", err)
		}
	})
}

func TestNewResolvesExe(t *testing.T) {
	s := New("0.7.0")
	if s.version != "0.7.0" {
		t.Fatalf("version = %q", s.version)
	}
	// The asset download must not ride the short metadata timeout: a client
	// Timeout covers the whole body, and 5s cuts a multi-MiB binary mid-stream.
	if s.download == nil {
		t.Fatal("download client not set")
	}
	if s.download.Timeout <= s.http.Timeout {
		t.Fatalf("download timeout = %v, want longer than metadata %v", s.download.Timeout, s.http.Timeout)
	}
	if s.latestURL != latestReleaseURL {
		t.Fatalf("latestURL = %q", s.latestURL)
	}
	if s.goos != runtime.GOOS || s.goarch != runtime.GOARCH {
		t.Fatalf("platform = %s/%s, want this build's %s/%s", s.goos, s.goarch, runtime.GOOS, runtime.GOARCH)
	}
	exe, _ := os.Executable()
	if s.exePath != exe {
		t.Fatalf("exePath = %q, want %q", s.exePath, exe)
	}
}
