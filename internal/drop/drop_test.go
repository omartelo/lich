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
	"runtime"
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

	got := newService(t).Resolve(root, []Item{item}, false)

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
			if got := newService(t).Resolve(root, []Item{dropped}, false); got[0] != "" {
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

	if got := newService(t).Resolve(root, []Item{item}, false); got[0] == "" {
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

	if got := newService(t).Resolve(root, []Item{item}, false); got[0] != "" {
		t.Fatalf("Resolve = %q, want no match for twins", got[0])
	}
}

// TestResolveRefusesTwinsSplitByTheBudget is the twin rule at the one place it
// could quietly break: the budget dies inside a level, so one twin was seen and
// the other never will be. A lone candidate in a level nobody finished reading
// is not a unique match, and answering with it hands the session the wrong file
// — the exact failure the whole rule exists to prevent.
func TestResolveRefusesTwinsSplitByTheBudget(t *testing.T) {
	root := t.TempDir()
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	item := writeFile(t, filepath.Join(root, "a", "app.ts"), 41, mtime)
	writeFile(t, filepath.Join(root, "b", "app.ts"), 41, mtime)
	// Noise inside the first directory only, named to sort after the twin it
	// hides behind: the hit lands, then the budget runs out before the level's
	// second directory is read at all.
	for i := 0; i <= maxEntries; i++ {
		name := filepath.Join(root, "a", fmt.Sprintf("z%d", i))
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatalf("write noise: %v", err)
		}
	}

	if got := newService(t).Resolve(root, []Item{item}, false); got[0] != "" {
		t.Fatalf("Resolve = %q, want no match for twins the budget split", got[0])
	}
}

// TestResolveSurvivesAnUnreadableDir: a directory the user cannot read costs
// its own items their path and nothing else — the sibling tree is still
// searched, and the walk does not end there.
func TestResolveSurvivesAnUnreadableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory mode bits do not gate reads on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root reads a directory whatever its mode")
	}
	root := t.TempDir()
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	// Sorts before the readable sibling, so the failed read happens first.
	locked := filepath.Join(root, "a-locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	want := filepath.Join(root, "b-src", "app.ts")
	item := writeFile(t, want, 41, mtime)

	if got := newService(t).Resolve(root, []Item{item}, false); got[0] != want {
		t.Fatalf("Resolve = %q, want %q", got[0], want)
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

	if got := newService(t).Resolve(root, []Item{item}, false); got[0] != want {
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

	if got := newService(t).Resolve(root, []Item{item}, false); got[0] != want {
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

	got := newService(t).Resolve(root, []Item{first, elsewhere, third}, false)

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

	if got := newService(t).Resolve(root, []Item{item}, false); got[0] != want {
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

	got := service.Resolve(t.TempDir(), []Item{{Name: "notes", Dir: true}}, false)

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

	got := homeAt(t, home).Resolve(root, []Item{inSession, inHome}, false)

	if want := filepath.Join(root, ".github", "ci.yml"); got[0] != want {
		t.Fatalf("session hidden dir = %q, want %q", got[0], want)
	}
	if got[1] != "" {
		t.Fatalf("home hidden dir = %q, want no match", got[1])
	}
}

func TestResolveEmptyRoot(t *testing.T) {
	if got := newService(t).Resolve("", []Item{{Name: "app.ts"}}, false); len(got) != 1 || got[0] != "" {
		t.Fatalf("Resolve = %v, want one empty path", got)
	}
}

// canConfine fixes what the host would answer about the sandbox, so the tests
// below describe a confined session on any machine — with or without
// bubblewrap installed, on any OS.
func canConfine(t *testing.T, available bool) {
	t.Helper()
	previous := sandboxAvailable
	sandboxAvailable = func() bool { return available }
	t.Cleanup(func() { sandboxAvailable = previous })
}

// TestResolveSkipsHomeForAConfinedSession is the whole point of the flag: the
// file is there, under the user's real home, and its path names nothing inside
// a sandbox whose home is empty. Answering with it would paste a path the agent
// cannot open, so the item is left for the copy instead.
func TestResolveSkipsHomeForAConfinedSession(t *testing.T) {
	canConfine(t, true)
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	home := t.TempDir()
	item := writeFile(t, filepath.Join(home, "Downloads", "spec.pdf"), 20, mtime)

	got := homeAt(t, home).Resolve(t.TempDir(), []Item{item}, true)

	if got[0] != "" {
		t.Fatalf("Resolve = %q, want no path — the sandbox has no such file", got[0])
	}
}

// TestResolveSearchesTheCheckoutOfAConfinedSession: the checkout is the one
// tree a confined session does see, so the real path is still the answer there.
func TestResolveSearchesTheCheckoutOfAConfinedSession(t *testing.T) {
	canConfine(t, true)
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	root := t.TempDir()
	want := filepath.Join(root, "src", "app.ts")
	item := writeFile(t, want, 41, mtime)

	got := newService(t).Resolve(root, []Item{item}, true)

	if got[0] != want {
		t.Fatalf("Resolve = %q, want %q", got[0], want)
	}
}

// TestResolveSearchesHomeWhereNothingConfines pins the other half of the
// answer, and with it the Windows behaviour: the row can say a session is
// confined on a machine that has no backend to confine it, and there the home
// is the session's own — searching it is what a drop there has always done.
func TestResolveSearchesHomeWhereNothingConfines(t *testing.T) {
	canConfine(t, false)
	mtime := time.Now().Add(-time.Hour).Truncate(time.Second)
	home := t.TempDir()
	want := filepath.Join(home, "Downloads", "spec.pdf")
	item := writeFile(t, want, 20, mtime)

	got := homeAt(t, home).Resolve(t.TempDir(), []Item{item}, true)

	if got[0] != want {
		t.Fatalf("Resolve = %q, want %q", got[0], want)
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

// TestUploadRefusesOtherMethods: the endpoint writes a file, so a method that
// is not the POST the page makes is refused rather than answered.
func TestUploadRefusesOtherMethods(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		t.Run(method, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			target := "/drop?name=shot.png&session=s1"
			New(t.TempDir()).Upload(recorder, httptest.NewRequest(method, target, nil))

			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s = %d, want %d", method, recorder.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

// TestUploadStoresBytes covers the whole endpoint: the page posts the file and
// gets back a path that holds exactly what it sent. The CORS header is asserted
// with it because in dev the page's origin is the Vite server, so a response
// without it is one the page is not allowed to read.
func TestUploadStoresBytes(t *testing.T) {
	dir := t.TempDir()
	recorder := httptest.NewRecorder()
	target := "/drop?name=shot.png&session=s1"
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader("bytes"))

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
	if want := filepath.Join(dir, "lich", "dropped", "s1", "shot.png"); answer.Path != want {
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
			target := "/drop?session=s1&name=" + url.QueryEscape(name)
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
				if want := filepath.Join(dir, "lich", "dropped", "s1", "passwd"); answer.Path != want {
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

// agedCopy puts a file in one session's copies directory with a chosen age,
// standing in for a copy an earlier drop left behind.
func agedCopy(t *testing.T, service *Service, session, name string, age time.Duration) string {
	t.Helper()
	dir := filepath.Join(service.dir, session)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, name)
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
// week or a month.
func TestPruneDeletesOnlyExpiredCopies(t *testing.T) {
	service := New(t.TempDir())
	expired := agedCopy(t, service, "s1", "old.png", 4*24*time.Hour)
	kept := agedCopy(t, service, "s1", "recent.png", 2*24*time.Hour)

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
	expired := agedCopy(t, service, "s1", "old.png", 4*24*time.Hour)

	fresh, err := service.Save("s1", "new.png", strings.NewReader("bytes"))
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

// failingReader gives some bytes and then stops, like a request the page
// aborts halfway through.
type failingReader struct{ read bool }

func (r *failingReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, errors.New("connection went away")
	}
	r.read = true
	return copy(p, "half"), nil
}

// TestSaveLeavesNothingBehindOnFailure: the caller is told the copy failed, so
// no path was ever pasted — a truncated file under that name would be waste
// the next drop of the same file has to route around.
func TestSaveLeavesNothingBehindOnFailure(t *testing.T) {
	service := New(t.TempDir())

	if _, err := service.Save("s1", "shot.png", &failingReader{}); err == nil {
		t.Fatal("Save reported success on a body that failed")
	}

	entries, err := os.ReadDir(filepath.Join(service.dir, "s1"))
	if err != nil {
		t.Fatalf("read copies dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("copies dir holds %d entries, want none", len(entries))
	}
}

// TestPruneWithoutCopiesDir covers a copies directory that is not there — the
// user cleared it, or the config directory is on a volume that went away. Not
// a fault, and not something a prune may report as one.
func TestPruneWithoutCopiesDir(t *testing.T) {
	service := New(t.TempDir())
	if err := os.RemoveAll(service.dir); err != nil {
		t.Fatalf("remove copies dir: %v", err)
	}

	service.Prune()
	service.PruneStale()
}

// TestSaveKeepsEarlierDrops pins the no-overwrite rule: the first copy's path
// may still be sitting unsent in a prompt when the second file lands.
func TestSaveKeepsEarlierDrops(t *testing.T) {
	service := New(t.TempDir())

	first, err := service.Save("s1", "shot.png", strings.NewReader("one"))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	second, err := service.Save("s1", "shot.png", strings.NewReader("two"))
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

// TestSaveRefusesAnExhaustedName is the far end of the no-overwrite rule: with
// every suffix taken there is no path left to hand out, and the drop has to
// fail rather than truncate the copy holding that name. The 999 names are
// written as literals — deriving them from maxCopies would follow the constant
// wherever it moved and never fail.
func TestSaveRefusesAnExhaustedName(t *testing.T) {
	service := New(t.TempDir())
	dir := filepath.Join(service.dir, "s1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	last := filepath.Join(dir, "shot-999.png")
	for i := 1; i <= 999; i++ {
		name := "shot.png"
		if i > 1 {
			name = fmt.Sprintf("shot-%d.png", i)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("taken"), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	if _, err := service.Save("s1", "shot.png", strings.NewReader("new bytes")); err == nil {
		t.Fatal("Save reported success with every name taken")
	}

	if got, err := os.ReadFile(last); err != nil || string(got) != "taken" {
		t.Fatalf("last copy = %q (%v), want it untouched", got, err)
	}
}

// TestNewCreatesTheCopiesDir: a confined session binds this directory when it
// spawns, and a bind of a source that is not there is silently dropped — so the
// directory has to exist before any session opens, not after the first drop.
func TestNewCreatesTheCopiesDir(t *testing.T) {
	config := t.TempDir()

	service := New(config)

	info, err := os.Stat(service.dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("copies dir = %v (%v), want a directory at %s", info, err, service.dir)
	}
}

// TestUploadRejectsAMissingSession: without an id there is no directory that a
// confined session can read and no session whose close deletes the copy, so
// the drop is refused rather than written somewhere nothing ends.
func TestUploadRejectsAMissingSession(t *testing.T) {
	for _, session := range []string{"", "..", "."} {
		t.Run(session, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			target := "/drop?name=shot.png&session=" + url.QueryEscape(session)
			request := httptest.NewRequest(http.MethodPost, target, strings.NewReader("x"))

			New(t.TempDir()).Upload(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("upload = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
		})
	}
}

// TestUploadKeepsATraversingSessionInside: the id reaches the backend off a
// query string, so it is shaped by whatever sent it — and it may only ever name
// a directory inside the copies tree.
func TestUploadKeepsATraversingSessionInside(t *testing.T) {
	config := t.TempDir()
	recorder := httptest.NewRecorder()
	target := "/drop?name=shot.png&session=" + url.QueryEscape("../../evil")
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader("x"))

	New(config).Upload(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("upload = %d (%s), want %d", recorder.Code, recorder.Body, http.StatusOK)
	}
	var answer struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decode %s: %v", recorder.Body, err)
	}
	if want := filepath.Join(config, "lich", "dropped", "evil", "shot.png"); answer.Path != want {
		t.Fatalf("path = %q, want %q", answer.Path, want)
	}
}

// TestPurgeDeletesOneSessionsCopies is the lifetime the feature promises: the
// copies exist for the conversation they were dropped into, and the session
// closing is what ends them — with the copies of every other session untouched.
func TestPurgeDeletesOneSessionsCopies(t *testing.T) {
	service := New(t.TempDir())
	gone := agedCopy(t, service, "s1", "shot.png", time.Minute)
	kept := agedCopy(t, service, "s2", "shot.png", time.Minute)

	service.Purge("s1")

	if _, err := os.Stat(gone); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("purged session's copy still there (%v)", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("another session's copy was deleted: %v", err)
	}
}

// TestPurgeRefusesToLeaveTheCopiesDir: Purge is handed a session id, and the
// one thing it may never do is take a path out of the tree it owns.
func TestPurgeRefusesToLeaveTheCopiesDir(t *testing.T) {
	service := New(t.TempDir())
	kept := agedCopy(t, service, "s1", "shot.png", time.Minute)

	for _, session := range []string{"", ".", "..", "../"} {
		service.Purge(session)
	}

	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("copies dir emptied by a bad session id: %v", err)
	}
	if _, err := os.Stat(service.dir); err != nil {
		t.Fatalf("copies dir removed by a bad session id: %v", err)
	}
}

// TestPruneKeepsAnEmptySessionDir is the trap the second method exists for: a
// confined session binds its own copies directory at spawn, so removing it from
// under a live session would leave every later copy of that session invisible
// inside the sandbox. A drop by another session must not do that.
func TestPruneKeepsAnEmptySessionDir(t *testing.T) {
	service := New(t.TempDir())
	expired := agedCopy(t, service, "s1", "old.png", 4*24*time.Hour)
	dir := filepath.Dir(expired)

	service.Prune()

	if _, err := os.Stat(expired); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expired copy still there (%v)", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("session dir removed by a plain prune: %v", err)
	}
}

// TestPruneStaleRemovesEmptySessionDirs is the startup sweep: nothing has
// spawned yet, so a directory with nothing left in it belongs to a session that
// is over — the crash that never reported one gone.
func TestPruneStaleRemovesEmptySessionDirs(t *testing.T) {
	service := New(t.TempDir())
	expired := agedCopy(t, service, "s1", "old.png", 4*24*time.Hour)
	kept := agedCopy(t, service, "s2", "recent.png", time.Minute)

	service.PruneStale()

	if _, err := os.Stat(filepath.Dir(expired)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("emptied session dir still there (%v)", err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("copy inside the window was deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(kept)); err != nil {
		t.Fatalf("session dir with copies left in it was removed: %v", err)
	}
}
