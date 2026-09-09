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

func TestWindowsApplyAsset(t *testing.T) {
	for _, window := range []bool{false, true} {
		t.Run(fmt.Sprint(window), func(t *testing.T) {
			exe := filepath.Join(t.TempDir(), "lich.exe")
			asset := "lich-v0.8.0-windows-amd64.exe"
			if window {
				exe = windowedExe(t)
				asset = "lich-v0.8.0-windows-amd64-setup.exe"
			}
			body := "verified release asset"
			srv := windowsReleaseFixture(t, asset, body)
			defer srv.Close()
			s := applyService(t, srv, func(r io.Reader, checksum []byte) error {
				if window {
					t.Fatal("installer layout must not swap the bare exe")
				}
				return nil
			})
			s.goos, s.goarch, s.exePath = "windows", "amd64", exe
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
			if installed != window {
				t.Fatalf("installed = %v, window = %v", installed, window)
			}
		})
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
	if err := stageInstaller(strings.NewReader(body), sum[:], filepath.Join(t.TempDir(), "missing", "setup.exe")); err == nil {
		t.Fatal("unwritable staging directory must fail")
	}
	if err := stageInstaller(failingReader{}, sum[:], filepath.Join(t.TempDir(), "setup.exe")); err != io.ErrUnexpectedEOF {
		t.Fatalf("read error = %v", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
