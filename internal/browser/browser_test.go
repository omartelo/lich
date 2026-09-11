package browser

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testService(t *testing.T, launch *fakeLauncher) *Service {
	t.Helper()
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	s := newService(launch.launch, func(string, string) (string, error) {
		return t.TempDir(), nil
	}, func() time.Time { return now }, 5*time.Minute)
	s.now = func() time.Time { return now }
	t.Cleanup(func() { _ = s.Cleanup() })
	return s
}

func TestOpenCreatesOneHeadlessPerOwner(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)

	a, err := s.Open("sess-a", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if a.Headed || a.Owner != "sess-a" || a.URL != "https://example.com" {
		t.Fatalf("handle = %+v", a)
	}
	b, err := s.Open("sess-a", "https://example.com/next")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Fatalf("second Open minted a new browser: %s then %s", a.ID, b.ID)
	}
	if len(launch.pages) != 1 {
		t.Fatalf("launched %d, want 1", len(launch.pages))
	}
	if got := launch.pages[0].navigations; len(got) != 2 {
		t.Fatalf("navigations = %v", got)
	}
	c, err := s.Open("sess-b", "https://other.example")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == a.ID {
		t.Fatal("a second lich session shared the first's browser")
	}
	if len(launch.pages) != 2 {
		t.Fatalf("launched %d, want 2", len(launch.pages))
	}
}

func TestOpenRejectsBlockedURL(t *testing.T) {
	s := testService(t, &fakeLauncher{})
	if _, err := s.Open("s", "file:///etc/passwd"); err == nil {
		t.Fatal("file: URL was allowed")
	}
	list, err := s.List("s")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("a rejected Open still left a browser: %v", list)
	}
}

func TestOpenVisibleReusesHeadedAndReplacesHeadless(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)

	first, err := s.OpenVisible("s")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Headed {
		t.Fatal("OpenVisible launched headless")
	}
	second, err := s.OpenVisible("s")
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("second OpenVisible replaced a headed window")
	}
	if launch.pages[0].focused == 0 {
		t.Fatal("reuse did not focus the existing window")
	}

	h, err := s.Open("other", "https://example.com/page")
	if err != nil {
		t.Fatal(err)
	}
	if h.Headed {
		t.Fatal("Open without a window should be headless")
	}
	vis, err := s.OpenVisible("other")
	if err != nil {
		t.Fatal(err)
	}
	if vis.ID == h.ID {
		t.Fatal("OpenVisible kept the headless page instead of replacing it")
	}
	if !vis.Headed || vis.URL != "https://example.com/page" {
		t.Fatalf("replaced handle = %+v", vis)
	}
	if !launch.pages[1].closed {
		t.Fatal("headless page was not closed when promoting")
	}
}

func TestCloseOwnedByAndCleanup(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)
	if _, err := s.Open("a", "https://a.example"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open("b", "https://b.example"); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseOwnedBy("a"); err != nil {
		t.Fatal(err)
	}
	if !launch.pages[0].closed {
		t.Fatal("CloseOwnedBy left the page open")
	}
	list, _ := s.List("a")
	if len(list) != 0 {
		t.Fatal("owner a still has a browser")
	}
	if err := s.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if !launch.pages[1].closed {
		t.Fatal("Cleanup left a page open")
	}
}

func TestActionsNeedABrowserAndRecordOnThePage(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)
	if _, err := s.Info("missing"); !errors.Is(err, errNoBrowser) {
		t.Fatalf("Info with none: %v", err)
	}

	h, err := s.Open("s", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	idx := 3
	if err := s.Click(h.ID, Target{Index: &idx}); err != nil {
		t.Fatal(err)
	}
	if err := s.Type("s", "hello", true, Target{}); err != nil {
		t.Fatal(err)
	}
	if err := s.Press("s", "Enter", Target{}); err != nil {
		t.Fatal(err)
	}
	dest, err := s.Screenshot("s", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if filepath.Base(dest) != "screenshot.png" {
		t.Fatalf("screenshot path = %s", dest)
	}
	if err := s.Reload("s"); err != nil {
		t.Fatal(err)
	}
	if err := s.Back("s"); err != nil {
		t.Fatal(err)
	}
	if err := s.Forward("s"); err != nil {
		t.Fatal(err)
	}
	if err := s.Scroll("s", 400); err != nil {
		t.Fatal(err)
	}
	p := launch.pages[0]
	if len(p.clicks) != 1 || p.clicks[0].Index == nil || *p.clicks[0].Index != 3 {
		t.Fatalf("clicks = %+v", p.clicks)
	}
	if len(p.typed) != 1 || p.typed[0].text != "hello" || !p.typed[0].clear {
		t.Fatalf("typed = %+v", p.typed)
	}
	if len(p.pressed) != 1 || p.pressed[0].key != "Enter" {
		t.Fatalf("pressed = %+v", p.pressed)
	}
	if p.reloads != 1 || p.backs != 1 || p.forwards != 1 {
		t.Fatalf("nav = reload %d back %d forward %d", p.reloads, p.backs, p.forwards)
	}
	if len(p.scrolled) != 1 || p.scrolled[0] != 400 {
		t.Fatalf("scrolled = %v", p.scrolled)
	}
}

func TestClickRequiresATarget(t *testing.T) {
	s := testService(t, &fakeLauncher{})
	if _, err := s.Open("s", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Click("s", Target{}); err == nil {
		t.Fatal("click with no target succeeded")
	}
}

func TestPressRejectsUnknownKey(t *testing.T) {
	s := testService(t, &fakeLauncher{})
	if _, err := s.Open("s", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.Press("s", "Ctrl+A", Target{}); err == nil {
		t.Fatal("unknown key was allowed")
	}
	if err := s.Press("s", "", Target{}); err == nil {
		t.Fatal("empty key was allowed")
	}
}

func TestNavigateRejectsBlockedURL(t *testing.T) {
	s := testService(t, &fakeLauncher{})
	if _, err := s.Open("s", "https://ok.example"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Navigate("s", "javascript:alert(1)"); err == nil {
		t.Fatal("navigate allowed javascript:")
	}
}

func TestIdleReaperClosesUnused(t *testing.T) {
	launch := &fakeLauncher{}
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	s := newService(launch.launch, func(string, string) (string, error) {
		return t.TempDir(), nil
	}, func() time.Time { return now }, time.Minute)
	t.Cleanup(func() { _ = s.Cleanup() })

	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	s.Reap()
	if launch.pages[0].closed {
		t.Fatal("reaper closed a browser that was just used")
	}
	now = now.Add(time.Minute + time.Second)
	s.now = func() time.Time { return now }
	s.Reap()
	if !launch.pages[0].closed {
		t.Fatal("reaper left an idle browser running")
	}
}

func TestOpenRequiresOwner(t *testing.T) {
	s := testService(t, &fakeLauncher{})
	if _, err := s.Open("", "https://example.com"); !errors.Is(err, errNoOwner) {
		t.Fatalf("Open with no owner: %v", err)
	}
	if _, err := s.OpenVisible(""); !errors.Is(err, errNoOwner) {
		t.Fatalf("OpenVisible with no owner: %v", err)
	}
}

func TestNavigateUpdatesHandle(t *testing.T) {
	s := testService(t, &fakeLauncher{})
	if _, err := s.Open("s", "https://a.example"); err != nil {
		t.Fatal(err)
	}
	h, err := s.Navigate("s", "https://b.example")
	if err != nil {
		t.Fatal(err)
	}
	if h.URL != "https://b.example" {
		t.Fatalf("handle url = %q", h.URL)
	}
}

func TestScreenshotWritesTheAskedPath(t *testing.T) {
	s := testService(t, &fakeLauncher{})
	if _, err := s.Open("s", ""); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "shot.png")
	got, err := s.Screenshot("s", dest)
	if err != nil {
		t.Fatal(err)
	}
	if got != dest {
		t.Fatalf("path = %q, want %q", got, dest)
	}
}

func TestFailedInfoDropsTheBrowser(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].infoErr = errors.New("chrome gone")
	if _, err := s.Info("s"); err == nil {
		t.Fatal("Info succeeded on a dead page")
	}
	list, _ := s.List("s")
	if len(list) != 0 {
		t.Fatal("dead page was not dropped")
	}
}

func TestOpenDropsWhenReuseNavigateFails(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)
	if _, err := s.Open("s", "https://a.example"); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].navErr = errors.New("gone")
	if _, err := s.Open("s", "https://b.example"); err == nil {
		t.Fatal("Open reused a page that could not navigate")
	}
	if !launch.pages[0].closed {
		t.Fatal("failed reuse left the page open")
	}
}

func TestLaunchFailureLeavesNoBrowser(t *testing.T) {
	s := newService(func(string, bool) (Page, error) {
		return nil, errors.New("no chrome")
	}, func(string, string) (string, error) {
		return t.TempDir(), nil
	}, time.Now, time.Minute)
	t.Cleanup(func() { _ = s.Cleanup() })
	if _, err := s.Open("s", "https://example.com"); err == nil {
		t.Fatal("Open succeeded without a launcher")
	}
	list, _ := s.List("s")
	if len(list) != 0 {
		t.Fatal("a failed launch left a handle")
	}
}

func TestCloseByHandleID(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)
	h, err := s.Open("s", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(h.ID); err != nil {
		t.Fatal(err)
	}
	if !launch.pages[0].closed {
		t.Fatal("Close(id) left the page open")
	}
}

func TestInfoReturnsPageInfoWithoutDroppingTheBrowser(t *testing.T) {
	launch := &fakeLauncher{}
	s := testService(t, launch)
	if _, err := s.Open("s", "https://example.com"); err != nil {
		t.Fatal(err)
	}
	launch.pages[0].title = "Example"
	launch.pages[0].info = PageInfo{Title: "Example", Text: "hello", Interactive: []Interactive{{Tag: "a", Text: "Home"}}}
	info, err := s.Info("s")
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Example" || info.Text != "hello" {
		t.Fatalf("info = %+v", info)
	}
	list, _ := s.List("s")
	if len(list) != 1 {
		t.Fatal("Info dropped the browser")
	}
}
