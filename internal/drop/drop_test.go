package drop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newService builds a Service whose home fallback is an empty directory, so a
// test that expects no match cannot be answered by the machine's real home.
// homeAt does the same with a home the test controls.
func newService(t *testing.T) *Service {
	t.Helper()
	return homeAt(t, t.TempDir())
}

func homeAt(t *testing.T, home string) *Service {
	t.Helper()
	previous := homeDir
	homeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { homeDir = previous })
	return New(t.TempDir())
}

// writeFile creates path with size bytes and a known modification time,
// standing in for the file the user drags out of a file manager.
func writeFile(t *testing.T, path string, size int, mtime time.Time) Item {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
	return Item{Name: filepath.Base(path), Size: int64(size), Mtime: mtime.UnixMilli()}
}

// TestResolveFindsFileByMetadata proves the one thing the whole feature rests
// on: name, size and mtime are enough to turn a pathless drop back into the
// user's own file, nested dirs included.
func TestResolveFindsFileByMetadata(t *testing.T) {
	root := t.TempDir()
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	want := filepath.Join(root, "src", "app.ts")
	item := writeFile(t, want, 41, mtime)

	got := newService(t).Resolve(root, []Item{item})

	if len(got) != 1 || got[0] != want {
		t.Fatalf("Resolve = %v, want [%s]", got, want)
	}
}

// TestResolveRejectsNearMiss pins the fields that must disagree for a match to
// be refused. Sizes are literals a mutation cannot make agree by widening the
// slack, and the stale mtime is a whole minute past it.
func TestResolveRejectsNearMiss(t *testing.T) {
	root := t.TempDir()
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	item := writeFile(t, filepath.Join(root, "app.ts"), 41, mtime)

	cases := map[string]Item{
		"wrong size":  {Name: "app.ts", Size: 42, Mtime: item.Mtime},
		"stale mtime": {Name: "app.ts", Size: 41, Mtime: mtime.Add(-time.Minute).UnixMilli()},
		"wrong name":  {Name: "other.ts", Size: 41, Mtime: item.Mtime},
		"file as dir": {Name: "app.ts", Size: 41, Mtime: item.Mtime, Dir: true},
	}
	for name, dropped := range cases {
		t.Run(name, func(t *testing.T) {
			if got := newService(t).Resolve(root, []Item{dropped}); got[0] != "" {
				t.Fatalf("Resolve = %q, want no match", got[0])
			}
		})
	}
}

// TestResolveAcceptsMtimeWithinSlack keeps the browser's millisecond clock and
// the filesystem's from having to agree exactly.
func TestResolveAcceptsMtimeWithinSlack(t *testing.T) {
	root := t.TempDir()
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	item := writeFile(t, filepath.Join(root, "app.ts"), 41, mtime)
	item.Mtime += mtimeSlackMs - 1

	if got := newService(t).Resolve(root, []Item{item}); got[0] == "" {
		t.Fatal("Resolve found nothing, want the file within the mtime slack")
	}
}

// TestResolveRefusesTwins is the safety rule: two identical files under the
// tree make the drop ambiguous, and pasting the wrong one silently edits the
// wrong file. No path is the correct answer — the caller falls back to a copy.
func TestResolveRefusesTwins(t *testing.T) {
	root := t.TempDir()
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	item := writeFile(t, filepath.Join(root, "a", "app.ts"), 41, mtime)
	writeFile(t, filepath.Join(root, "b", "app.ts"), 41, mtime)

	if got := newService(t).Resolve(root, []Item{item}); got[0] != "" {
		t.Fatalf("Resolve = %q, want no match for twins", got[0])
	}
}

// TestResolveSkipsNoiseDirs proves the skip list is a search boundary, not a
// speed-up: a match found inside .git would be the wrong file to hand a
// session, and it must not even count towards ambiguity.
func TestResolveSkipsNoiseDirs(t *testing.T) {
	root := t.TempDir()
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	want := filepath.Join(root, "app.ts")
	item := writeFile(t, want, 41, mtime)
	writeFile(t, filepath.Join(root, ".git", "app.ts"), 41, mtime)
	writeFile(t, filepath.Join(root, "node_modules", "pkg", "app.ts"), 41, mtime)

	if got := newService(t).Resolve(root, []Item{item}); got[0] != want {
		t.Fatalf("Resolve = %q, want %q", got[0], want)
	}
}

// TestResolveMatchesDirectoryByName covers the item the page cannot describe:
// a dropped directory reports a placeholder size, so the name is all there is.
func TestResolveMatchesDirectoryByName(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "src", "components")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	item := Item{Name: "components", Size: 4096, Mtime: 0, Dir: true}

	if got := newService(t).Resolve(root, []Item{item}); got[0] != want {
		t.Fatalf("Resolve = %q, want %q", got[0], want)
	}
}

// TestResolveMapsEveryItem proves a batch drop keeps its order and reports the
// unfound ones in place, so the caller can pair path to item by index.
func TestResolveMapsEveryItem(t *testing.T) {
	root := t.TempDir()
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	first := writeFile(t, filepath.Join(root, "a.ts"), 10, mtime)
	third := writeFile(t, filepath.Join(root, "c.ts"), 30, mtime)
	elsewhere := Item{Name: "b.ts", Size: 20, Mtime: mtime.UnixMilli()}

	got := newService(t).Resolve(root, []Item{first, elsewhere, third})

	if len(got) != 3 || got[0] == "" || got[1] != "" || got[2] == "" {
		t.Fatalf("Resolve = %v, want the outer two found and the middle empty", got)
	}
}

// TestResolveFindsShallowSiblingBeforeNoise is the measured regression: a home
// directory's caches sort before an ordinary name and hold tens of thousands
// of files, so a depth-first search spent its whole budget inside them and
// reported the directory sitting right beside it as missing. The noise here is
// one entry over the budget for exactly that reason.
func TestResolveFindsShallowSiblingBeforeNoise(t *testing.T) {
	root := t.TempDir()
	noise := filepath.Join(root, ".cache", "pkg")
	if err := os.MkdirAll(noise, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for i := 0; i <= maxEntries; i++ {
		if err := os.WriteFile(filepath.Join(noise, fmt.Sprint(i)), nil, 0o644); err != nil {
			t.Fatalf("write noise: %v", err)
		}
	}
	want := filepath.Join(root, "project")
	if err := os.Mkdir(want, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	item := Item{Name: "project", Dir: true}

	if got := newService(t).Resolve(root, []Item{item}); got[0] != want {
		t.Fatalf("Resolve = %q, want %q", got[0], want)
	}
}

// TestResolveFallsBackToHome covers the drop a session's own tree can never
// answer: a directory from elsewhere, which has no copy to fall back on.
func TestResolveFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, "notes")
	if err := os.Mkdir(want, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	service := homeAt(t, home)

	got := service.Resolve(t.TempDir(), []Item{{Name: "notes", Dir: true}})

	if got[0] != want {
		t.Fatalf("Resolve = %q, want %q", got[0], want)
	}
}

// TestResolveSearchesHiddenDirsOfTheSessionOnly draws the line between the two
// trees: .github is part of a checkout and worth searching, while home's dot
// directories are caches whose size is the reason the search is bounded.
func TestResolveSearchesHiddenDirsOfTheSessionOnly(t *testing.T) {
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	root := t.TempDir()
	inSession := writeFile(t, filepath.Join(root, ".github", "ci.yml"), 12, mtime)
	home := t.TempDir()
	inHome := writeFile(t, filepath.Join(home, ".cache", "blob.bin"), 13, mtime)

	got := homeAt(t, home).Resolve(root, []Item{inSession, inHome})

	if want := filepath.Join(root, ".github", "ci.yml"); got[0] != want {
		t.Fatalf("session hidden dir = %q, want %q", got[0], want)
	}
	if got[1] != "" {
		t.Fatalf("home hidden dir = %q, want no match", got[1])
	}
}

func TestResolveEmptyRoot(t *testing.T) {
	if got := newService(t).Resolve("", []Item{{Name: "app.ts"}}); len(got) != 1 || got[0] != "" {
		t.Fatalf("Resolve = %v, want one empty path", got)
	}
}

// TestUploadAnswersPreflight is the one the browser asks first: the body
// carries the dropped file's own content type, which is not a type a simple
// request may have, so a POST that skips this never leaves the page.
func TestUploadAnswersPreflight(t *testing.T) {
	recorder := httptest.NewRecorder()

	New(t.TempDir()).Upload(recorder, httptest.NewRequest(http.MethodOptions, "/drop", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("preflight Allow-Origin = %q, want %q", got, "*")
	}
}

// TestUploadStoresBytes covers the whole endpoint: the page posts the file and
// gets back a path that holds exactly what it sent. The CORS header is asserted
// with it because in dev the page's origin is the Vite server, so a response
// without it is one the page is not allowed to read.
func TestUploadStoresBytes(t *testing.T) {
	dir := t.TempDir()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/drop?name=shot.png", strings.NewReader("bytes"))

	New(dir).Upload(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("upload = %d (%s), want %d", recorder.Code, recorder.Body, http.StatusOK)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Allow-Origin = %q, want %q", got, "*")
	}
	var answer struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decode %s: %v", recorder.Body, err)
	}
	if want := filepath.Join(dir, "lich", "dropped", "shot.png"); answer.Path != want {
		t.Fatalf("path = %q, want %q", answer.Path, want)
	}
	if got, err := os.ReadFile(answer.Path); err != nil || string(got) != "bytes" {
		t.Fatalf("stored = %q (%v), want %q", got, err, "bytes")
	}
}

// TestUploadRejectsBadName keeps a dropped name from steering the write out of
// the copies directory — the name is attacker-shaped input in the sense that
// nothing about a drop guarantees its shape.
func TestUploadRejectsBadName(t *testing.T) {
	for _, name := range []string{"", "..", ".", "../../etc/passwd", "/etc/passwd"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			recorder := httptest.NewRecorder()
			target := "/drop?name=" + url.QueryEscape(name)
			request := httptest.NewRequest(http.MethodPost, target, strings.NewReader("x"))

			New(dir).Upload(recorder, request)

			if name == "../../etc/passwd" || name == "/etc/passwd" {
				// Base() reduces a traversal to its last element, which is a
				// legitimate file name — it must land inside the directory.
				if recorder.Code != http.StatusOK {
					t.Fatalf("upload = %d, want %d", recorder.Code, http.StatusOK)
				}
				var answer struct {
					Path string `json:"path"`
				}
				if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
					t.Fatalf("decode: %v", err)
				}
				if want := filepath.Join(dir, "lich", "dropped", "passwd"); answer.Path != want {
					t.Fatalf("path = %q, want %q", answer.Path, want)
				}
				return
			}
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("upload = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}

// agedCopy puts a file in the copies directory with a chosen age, standing in
// for a copy an earlier drop left behind.
func agedCopy(t *testing.T, service *Service, name string, age time.Duration) string {
	t.Helper()
	if err := os.MkdirAll(service.dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(service.dir, name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	stamp := time.Now().Add(-age)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
	return path
}

// TestPruneDeletesOnlyExpiredCopies pins both sides of the deadline. The ages
// are literal days, not keepDropped ± something: a test that derives them from
// the constant moves with it, and would stay green for a window widened to a
// month.
func TestPruneDeletesOnlyExpiredCopies(t *testing.T) {
	service := New(t.TempDir())
	expired := agedCopy(t, service, "old.png", 8*24*time.Hour)
	kept := agedCopy(t, service, "recent.png", 6*24*time.Hour)

	service.Prune()

	if _, err := os.Stat(expired); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expired copy still there (%v)", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("copy inside the window was deleted: %v", err)
	}
}

// TestSavePrunes is the trigger that matters most: a lich left running for
// weeks never restarts, so the copy that grows the directory is what has to
// clear the ones before it.
func TestSavePrunes(t *testing.T) {
	service := New(t.TempDir())
	expired := agedCopy(t, service, "old.png", 8*24*time.Hour)

	fresh, err := service.Save("new.png", strings.NewReader("bytes"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(expired); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Save left the expired copy behind (%v)", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("Save pruned the copy it had just written: %v", err)
	}
}

// TestPruneWithoutCopiesDir covers the first launch: nothing has been dropped,
// so the directory does not exist, and that is not a fault.
func TestPruneWithoutCopiesDir(t *testing.T) {
	New(t.TempDir()).Prune()
}

// TestSaveKeepsEarlierDrops pins the no-overwrite rule: the first copy's path
// may still be sitting unsent in a prompt when the second file lands.
func TestSaveKeepsEarlierDrops(t *testing.T) {
	service := New(t.TempDir())

	first, err := service.Save("shot.png", strings.NewReader("one"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	second, err := service.Save("shot.png", strings.NewReader("two"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if first == second {
		t.Fatalf("Save reused %s for a second drop", first)
	}
	if got, err := os.ReadFile(first); err != nil || string(got) != "one" {
		t.Fatalf("first copy = %q (%v), want %q", got, err, "one")
	}
	if got, err := os.ReadFile(second); err != nil || string(got) != "two" {
		t.Fatalf("second copy = %q (%v), want %q", got, err, "two")
	}
	if want := "shot-2.png"; filepath.Base(second) != want {
		t.Fatalf("second copy named %q, want %q", filepath.Base(second), want)
	}
}
