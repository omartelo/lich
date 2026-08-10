package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/rage"
)

// rageClient runs `lich rage` against a stub collector: the real one scans
// PATH, runs the provider CLIs and asks GitHub for the plugin release, none of
// which a unit test may do. roots records the archive root it was asked for.
func rageClient(t *testing.T, err error, roots *[]string) (*client, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	c := &client{
		env:    func(string) string { return "" },
		stdout: &stdout,
		stderr: &stderr,
		bundle: func(w io.Writer, root string) (rage.Report, error) {
			*roots = append(*roots, root)
			_, _ = io.WriteString(w, "archive bytes")
			return rage.Report{Version: "v1.2.3", Platform: "linux/amd64", Instance: "not running"}, err
		},
	}
	return c, &stdout, &stderr
}

func TestRageWritesATimestampedBundleAndSaysWhatItHolds(t *testing.T) {
	t.Chdir(t.TempDir())
	var roots []string
	c, stdout, stderr := rageClient(t, nil, &roots)

	if code := dispatch([]string{"rage"}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	written := bundlesIn(t, ".")
	if len(written) != 1 {
		t.Fatalf("wrote %v, want one lich-rage-<timestamp>.tar.gz", written)
	}
	if !strings.HasPrefix(written[0], "lich-rage-") {
		t.Errorf("bundle is named %q", written[0])
	}
	if len(roots) != 1 || roots[0]+".tar.gz" != written[0] {
		t.Errorf("archive root %v does not match the file name %q", roots, written[0])
	}
	for _, want := range []string{written[0], "v1.2.3", "linux/amd64", "not running", "Read it before you attach it"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout does not mention %q: %q", want, stdout.String())
		}
	}
}

func TestRageWritesWhereItWasTold(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.tar.gz")
	var roots []string
	c, _, stderr := rageClient(t, nil, &roots)

	if code := dispatch([]string{"rage", "--output", path}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the bundle: %v", err)
	}
	if string(data) != "archive bytes" {
		t.Errorf("bundle = %q", data)
	}
	if len(roots) != 1 || roots[0] != "report" {
		t.Errorf("archive root = %v, want the file's own name", roots)
	}
}

func TestRageLeavesNoHalfWrittenBundleBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.tar.gz")
	var roots []string
	c, _, stderr := rageClient(t, errors.New("disk full"), &roots)

	if code := dispatch([]string{"rage", "--output", path}, c); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "disk full") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a failed collection left an archive that opens and is missing the part that mattered")
	}
}

func TestRageTakesNoArguments(t *testing.T) {
	t.Chdir(t.TempDir())
	var roots []string
	c, _, stderr := rageClient(t, nil, &roots)

	if code := dispatch([]string{"rage", "everything"}, c); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lich rage") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if written := bundlesIn(t, "."); len(written) != 0 {
		t.Errorf("a rejected command still wrote %v", written)
	}
}

func TestRageIsInTheHelp(t *testing.T) {
	f := newFakeLich(t, `null`)
	_, stdout, _ := run(t, f, "help")
	if !strings.Contains(stdout, "lich rage") {
		t.Errorf("help does not document rage: %q", stdout)
	}
}

func TestArchiveRootStripsBothExtensions(t *testing.T) {
	cases := map[string]string{
		"lich-rage-20260810-150405.tar.gz": "lich-rage-20260810-150405",
		"/tmp/report.tar.gz":               "report",
		"/tmp/report.tar":                  "report",
		"bundle":                           "bundle",
	}
	for path, want := range cases {
		if got := archiveRoot(path); got != want {
			t.Errorf("archiveRoot(%q) = %q, want %q", path, got, want)
		}
	}
}

func bundlesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}
