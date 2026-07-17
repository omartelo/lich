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
	}
	for _, tc := range tests {
		if got := canSelfApply(tc.goos, tc.exePath); got != tc.want {
			t.Errorf("canSelfApply(%q,%q) = %v, want %v", tc.goos, tc.exePath, got, tc.want)
		}
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
}

func TestApplyRejectedWhenNotSelfApply(t *testing.T) {
	s := New("0.7.0")
	s.exePath = "" // forces canSelfApply false regardless of platform
	if err := s.Apply(); err == nil {
		t.Fatal("Apply() = nil, want an error when self-apply is unsupported")
	}
}

// applyServer serves the release endpoints Apply hits: the latest-tag JSON,
// checksums.txt (with a matching hash for asset), and the asset bytes.
func applyServer(t *testing.T, asset string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/latest"):
			_, _ = io.WriteString(w, `{"tag_name":"v0.8.0"}`)
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			_, _ = io.WriteString(w, "deadbeef  "+asset+"\n")
		default:
			_, _ = io.WriteString(w, "FAKEBINARY")
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestApplyDownloadsVerifiesAndSwaps(t *testing.T) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skipf("no darwin asset for %s", runtime.GOARCH)
	}
	asset := "lich-v0.8.0-darwin-" + runtime.GOARCH
	srv := applyServer(t, asset)

	var gotBody string
	var gotChecksum []byte
	s := &Service{
		http:         srv.Client(),
		goos:         "darwin", // drive the self-apply path off a Linux CI host
		exePath:      filepath.Join(t.TempDir(), "lich"),
		latestURL:    srv.URL + "/latest",
		downloadBase: srv.URL + "/dl/",
		applyBinary: func(r io.Reader, checksum []byte) error {
			b, _ := io.ReadAll(r)
			gotBody = string(b)
			gotChecksum = checksum
			return nil
		},
	}

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
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Skipf("no darwin asset for %s", runtime.GOARCH)
	}
	asset := "lich-v0.8.0-darwin-" + runtime.GOARCH
	srv := applyServer(t, asset)
	s := &Service{
		http:         srv.Client(),
		goos:         "darwin",
		exePath:      filepath.Join(t.TempDir(), "lich"),
		latestURL:    srv.URL + "/latest",
		downloadBase: srv.URL + "/dl/",
		applyBinary:  func(io.Reader, []byte) error { return io.ErrUnexpectedEOF },
	}
	if err := s.Apply(); err == nil {
		t.Fatal("Apply() = nil, want the swap error propagated")
	}
}

func TestFetchChecksum(t *testing.T) {
	body := "aaaa  lich-v0.8.0-darwin-arm64\nbbbb  other\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	s := &Service{http: srv.Client()}

	sum, err := s.fetchChecksum(srv.URL, "lich-v0.8.0-darwin-arm64")
	if err != nil {
		t.Fatalf("fetchChecksum() error: %v", err)
	}
	if len(sum) != 2 || sum[0] != 0xaa || sum[1] != 0xaa {
		t.Fatalf("sum = %x, want aaaa", sum)
	}

	if _, err := s.fetchChecksum(srv.URL, "missing-asset"); err == nil {
		t.Fatal("fetchChecksum(missing) = nil error, want failure")
	}
}

func TestNewResolvesExe(t *testing.T) {
	s := New("0.7.0")
	if s.version != "0.7.0" {
		t.Fatalf("version = %q", s.version)
	}
	if s.latestURL != latestReleaseURL {
		t.Fatalf("latestURL = %q", s.latestURL)
	}
	exe, _ := os.Executable()
	if s.exePath != exe {
		t.Fatalf("exePath = %q, want %q", s.exePath, exe)
	}
}
