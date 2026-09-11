package browser

import (
	"context"
	"fmt"
	"path/filepath"
)

var errNoBrowser = fmt.Errorf("no browser for this session — open one first")

func (s *Service) live(id string) (*session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reapLocked()
	sess := s.lookupLocked(id)
	if sess == nil {
		return nil, errNoBrowser
	}
	return sess, nil
}

func (s *Service) touch(sess *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID[sess.id] == sess {
		sess.lastUsed = s.now()
	}
}

func (s *Service) drop(sess *session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID[sess.id] == sess {
		_ = s.destroyLocked(sess)
	}
}

func (s *Service) remember(sess *session, url, title string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byID[sess.id] != sess {
		return
	}
	if url != "" {
		sess.url = url
	}
	sess.title = title
	sess.lastUsed = s.now()
}

// Info returns the page the agent is looking at. Never includes form values.
func (s *Service) Info(id string) (PageInfo, error) {
	sess, err := s.live(id)
	if err != nil {
		return PageInfo{}, err
	}
	var info PageInfo
	err = runPage(func(ctx context.Context) error {
		var e error
		info, e = sess.page.Info(ctx)
		return e
	})
	if err != nil {
		s.drop(sess)
		return PageInfo{}, err
	}
	s.remember(sess, info.URL, info.Title)
	return info, nil
}

// Click sends a real mouse event at the target, not element.click().
func (s *Service) Click(id string, t Target) error {
	if t.empty() {
		return fmt.Errorf("click needs an index, a selector, or x and y")
	}
	return s.act(id, func(p Page, ctx context.Context) error {
		return p.Click(ctx, t)
	})
}

// Type sends keystrokes. clear selects-and-replaces; the typed text never
// comes back through Info.
func (s *Service) Type(id, text string, clear bool, t Target) error {
	return s.act(id, func(p Page, ctx context.Context) error {
		return p.Type(ctx, text, clear, t)
	})
}

// Press sends one named key (Enter, Tab, Escape, …) as a real key event.
// An optional target is focused first, the same way Type focuses before typing.
func (s *Service) Press(id, key string, t Target) error {
	if _, err := keySequence(key); err != nil {
		return err
	}
	return s.act(id, func(p Page, ctx context.Context) error {
		return p.Press(ctx, key, t)
	})
}

// Screenshot writes a PNG to dest (created if empty: inside the profile dir)
// and returns that path. The bytes stay off the transcript.
func (s *Service) Screenshot(id, dest string) (string, error) {
	sess, err := s.live(id)
	if err != nil {
		return "", err
	}
	if dest == "" {
		dest = filepath.Join(sess.dir, "screenshot.png")
	} else {
		abs, err := filepath.Abs(dest)
		if err != nil {
			return "", err
		}
		dest = abs
	}
	err = runPage(func(ctx context.Context) error {
		return sess.page.Screenshot(ctx, dest)
	})
	if err != nil {
		s.drop(sess)
		return "", err
	}
	s.touch(sess)
	return dest, nil
}

// Navigate loads url in this session's browser.
func (s *Service) Navigate(id, rawURL string) (Handle, error) {
	u, err := allowURL(rawURL)
	if err != nil {
		return Handle{}, err
	}
	sess, err := s.live(id)
	if err != nil {
		return Handle{}, err
	}
	err = runPage(func(ctx context.Context) error {
		return sess.page.Navigate(ctx, u)
	})
	if err != nil {
		s.drop(sess)
		return Handle{}, err
	}
	s.remember(sess, u, "")
	return sess.handle(), nil
}

// Reload reloads the current page.
func (s *Service) Reload(id string) error {
	return s.act(id, func(p Page, ctx context.Context) error { return p.Reload(ctx) })
}

// Back goes back in history.
func (s *Service) Back(id string) error {
	return s.act(id, func(p Page, ctx context.Context) error { return p.Back(ctx) })
}

// Forward goes forward in history.
func (s *Service) Forward(id string) error {
	return s.act(id, func(p Page, ctx context.Context) error { return p.Forward(ctx) })
}

// Scroll moves the viewport by dy CSS pixels (positive is down).
func (s *Service) Scroll(id string, dy int) error {
	return s.act(id, func(p Page, ctx context.Context) error { return p.Scroll(ctx, dy) })
}

func (s *Service) act(id string, fn func(Page, context.Context) error) error {
	sess, err := s.live(id)
	if err != nil {
		return err
	}
	err = runPage(func(ctx context.Context) error { return fn(sess.page, ctx) })
	if err != nil {
		s.drop(sess)
		return err
	}
	s.touch(sess)
	return nil
}
