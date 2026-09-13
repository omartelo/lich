package appupdate

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Windows install, from the installer or the portable zip, updates by
// verifying the release's installer and handing it over.
func TestWindowsApplyRunsTheInstaller(t *testing.T) {
	body := "verified release asset"
	srv := windowsReleaseFixture(t, applyAsset, body)
	defer srv.Close()
	s := applyService(t, srv)
	s.exePath = windowedExe(t)
	s.downloadBase = srv.URL + "/"
	installed := false
	s.install = func(path string) error {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != body {
			t.Fatalf("installer bytes = %q, %v", data, err)
		}
		installed = true
		return nil
	}
	if err := s.Apply(); err != nil {
		t.Fatal(err)
	}
	if !installed {
		t.Fatal("the installer did not run")
	}
}

func windowsReleaseFixture(t *testing.T, asset, body string) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256([]byte(body))
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			fmt.Fprint(w, `{"tag_name":"v0.8.0"}`)
		case "/v0.8.0/checksums.txt":
			fmt.Fprintf(w, "%x  %s\n", sum, asset)
		case "/v0.8.0/" + asset:
			fmt.Fprint(w, body)
		default:
			t.Errorf("unexpected asset request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func TestInstallerVerificationAndFailures(t *testing.T) {
	body := "installer"
	sum := sha256.Sum256([]byte(body))
	if canSelfApply("windows", "amd64", scoopExe(t)) {
		t.Fatal("a Scoop install must not run the Inno Setup installer")
	}
	s := &Service{exePath: windowedExe(t)}
	if err := s.applyInstaller(strings.NewReader(body), sum[:]); err == nil {
		t.Fatal("missing installer callback must fail")
	}
	launched := false
	s.install = func(string) error { launched = true; return io.ErrClosedPipe }
	if err := s.applyInstaller(strings.NewReader("corrupt"), sum[:]); err == nil || launched {
		t.Fatalf("bad checksum: err=%v, launched=%v", err, launched)
	}
	if err := s.applyInstaller(strings.NewReader(body), sum[:]); err != io.ErrClosedPipe {
		t.Fatalf("launch error = %v", err)
	}
	// A retry replaces the retained installer rather than accumulating downloads.
	if err := s.applyInstaller(strings.NewReader(body), sum[:]); err != io.ErrClosedPipe {
		t.Fatalf("retry error = %v", err)
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(s.exePath), ".lich-*.exe"))
	if err != nil || len(files) != 1 {
		t.Fatalf("retained downloads = %v, %v", files, err)
	}
	if err := stageInstaller(strings.NewReader(body), sum[:], filepath.Join(t.TempDir(), "missing", "setup.exe"), installerLimit); err == nil {
		t.Fatal("unwritable staging directory must fail")
	}
	if err := stageInstaller(failingReader{}, sum[:], filepath.Join(t.TempDir(), "setup.exe"), installerLimit); err != io.ErrUnexpectedEOF {
		t.Fatalf("read error = %v", err)
	}
}

func TestInstallerSizeCap(t *testing.T) {
	body := "installer"
	sum := sha256.Sum256([]byte(body))
	limit := int64(len(body))

	path := filepath.Join(t.TempDir(), "setup.exe")
	if err := stageInstaller(strings.NewReader(body), sum[:], path, limit); err != nil {
		t.Fatalf("an asset exactly at the limit must stage: %v", err)
	}
	if err := stageInstaller(strings.NewReader(body+"!"), sum[:], path, limit); err == nil {
		t.Fatal("an asset past the limit must be refused")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
