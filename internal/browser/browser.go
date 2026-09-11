package browser

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// idleAfter is how long a session's browser sits unused before the reaper
// closes it. Five minutes matches the "walked away from the card" window
// without holding a Chromium for a session that finished hours ago.
const idleAfter = 5 * time.Minute

// reapEvery is how often the reaper looks. Shorter than idleAfter so a
// browser that went idle is gone within a fraction of the idle window, not a
// full extra one.
const reapEvery = 30 * time.Second

// pageWait bounds one CDP action. Under the 90 seconds an MCP tool call may
// block for, long enough for a cold page, short enough that a hung Chrome
// does not take the tool with it.
const pageWait = 30 * time.Second

var errNoOwner = errors.New("browser needs a lich session id")

// Service is one Chromium context per lich session: the window Browser tab
// opens and the page the agent drives are the same one. Headless until
// someone asks for a window; headed after OpenVisible.
type Service struct {
	mu      sync.Mutex
	launch  Launcher
	mkTmp   func(dir, pattern string) (string, error)
	now     func() time.Time
	idle    time.Duration
	byOwner map[string]*session
	byID    map[string]*session
	stop    chan struct{}
	stopped bool
}

type session struct {
	id, owner, dir string
	url, title     string
	headed         bool
	page           Page
	lastUsed       time.Time
}

// New starts a service that launches the system Chromium (chromium.FindBrowser)
// into an ephemeral profile, never the lich --app one.
func New() *Service {
	s := newService(chromeLaunch, os.MkdirTemp, time.Now, idleAfter)
	s.startReaper()
	return s
}

func newService(launch Launcher, mkTmp func(string, string) (string, error), now func() time.Time, idle time.Duration) *Service {
	return &Service{
		launch:  launch,
		mkTmp:   mkTmp,
		now:     now,
		idle:    idle,
		byOwner: make(map[string]*session),
		byID:    make(map[string]*session),
		stop:    make(chan struct{}),
	}
}

// Open navigates this session's browser to url, creating a headless one if
// none exists. A headed window the user already opened is reused, not replaced.
func (s *Service) Open(owner, rawURL string) (Handle, error) {
	if owner == "" {
		return Handle{}, errNoOwner
	}
	u, err := allowURL(rawURL)
	if err != nil {
		return Handle{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked()
	if sess := s.liveByOwner(owner); sess != nil {
		if err := runPage(func(ctx context.Context) error {
			return sess.page.Navigate(ctx, u)
		}); err != nil {
			s.destroyLocked(sess)
			return Handle{}, err
		}
		sess.url = u
		sess.lastUsed = s.now()
		return sess.handle(), nil
	}
	return s.launchLocked(owner, u, false)
}

// OpenVisible brings this session's browser on screen. No browser yet: a
// headed window at about:blank. Already headed: focus it. Headless only:
// replace it with a headed window at the same URL — chromedp cannot promote
// a headless process in place.
func (s *Service) OpenVisible(owner string) (Handle, error) {
	if owner == "" {
		return Handle{}, errNoOwner
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked()
	if sess := s.liveByOwner(owner); sess != nil {
		if sess.headed {
			_ = runPage(sess.page.Focus)
			sess.lastUsed = s.now()
			return sess.handle(), nil
		}
		u := sess.url
		if u == "" {
			u = "about:blank"
		}
		s.destroyLocked(sess)
		return s.launchLocked(owner, u, true)
	}
	return s.launchLocked(owner, "about:blank", true)
}

// List returns this session's browser, or an empty list. Never null: a
// script should not have to tell those apart.
func (s *Service) List(owner string) ([]Handle, error) {
	if owner == "" {
		return nil, errNoOwner
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked()
	out := []Handle{}
	if sess := s.liveByOwner(owner); sess != nil {
		out = append(out, sess.handle())
	}
	return out, nil
}

// Close tears down one browser, by handle id or by the lich session that owns it.
func (s *Service) Close(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.lookupLocked(id)
	if sess == nil {
		return nil
	}
	return s.destroyLocked(sess)
}

// CloseOwnedBy tears down the browser of a lich session that is going away.
// Missing is not an error: most sessions never opened one.
func (s *Service) CloseOwnedBy(owner string) error {
	return s.Close(owner)
}

// Cleanup closes every browser this process started. Wired from main so a
// window-close exit does not leave Chromiums behind.
func (s *Service) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped {
		s.stopped = true
		close(s.stop)
	}
	var errs []error
	for _, sess := range s.byID {
		if err := s.destroyLocked(sess); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) launchLocked(owner, u string, headed bool) (Handle, error) {
	dir, err := s.mkTmp("", "lich-browser-")
	if err != nil {
		return Handle{}, err
	}
	page, err := s.launch(dir, headed)
	if err != nil {
		_ = os.RemoveAll(dir)
		return Handle{}, err
	}
	if err := runPage(func(ctx context.Context) error {
		return page.Navigate(ctx, u)
	}); err != nil {
		_ = page.Close()
		_ = os.RemoveAll(dir)
		return Handle{}, err
	}
	sess := &session{
		id: newID(), owner: owner, dir: dir, url: u, headed: headed, page: page, lastUsed: s.now(),
	}
	s.byOwner[owner] = sess
	s.byID[sess.id] = sess
	return sess.handle(), nil
}

func (s *Service) liveByOwner(owner string) *session {
	sess := s.byOwner[owner]
	if sess == nil || !sess.alive() {
		if sess != nil {
			s.destroyLocked(sess)
		}
		return nil
	}
	return sess
}

func (s *Service) lookupLocked(id string) *session {
	if sess := s.byID[id]; sess != nil {
		if sess.alive() {
			return sess
		}
		s.destroyLocked(sess)
	}
	if sess := s.byOwner[id]; sess != nil {
		if sess.alive() {
			return sess
		}
		s.destroyLocked(sess)
	}
	return nil
}

func (s *Service) destroyLocked(sess *session) error {
	delete(s.byID, sess.id)
	if s.byOwner[sess.owner] == sess {
		delete(s.byOwner, sess.owner)
	}
	err := sess.page.Close()
	if rmErr := os.RemoveAll(sess.dir); err == nil {
		err = rmErr
	}
	return err
}

func (s *session) alive() bool {
	return s != nil && s.page != nil && s.page.Alive()
}

func (s *session) handle() Handle {
	return Handle{ID: s.id, URL: s.url, Title: s.title, Owner: s.owner, Headed: s.headed}
}

func (s *Service) startReaper() {
	go func() {
		tick := time.NewTicker(reapEvery)
		defer tick.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-tick.C:
				s.Reap()
			}
		}
	}()
}

// Reap closes browsers that have sat idle past the idle window.
func (s *Service) Reap() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked()
}

func (s *Service) reapLocked() {
	cutoff := s.now().Add(-s.idle)
	for _, sess := range s.byID {
		if sess.lastUsed.Before(cutoff) {
			_ = s.destroyLocked(sess)
		}
	}
}

func runPage(fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), pageWait)
	defer cancel()
	return fn(ctx)
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
