package browser

import (
	"context"
	"os"
	"sync"
)

// fakePage is a Page that never talks to Chrome. Tests inject it through
// Launcher so the suite stays off the OS boundary.
type fakePage struct {
	mu          sync.Mutex
	url, title  string
	closed      bool
	navigations []string
	clicks      []Target
	typed       []typed
	pressed     []pressed
	reloads     int
	backs       int
	forwards    int
	scrolled    []int
	focused     int
	info        PageInfo
	shotErr     error
	navErr      error
	infoErr     error
}

type typed struct {
	text  string
	clear bool
	t     Target
}

type pressed struct {
	key string
	t   Target
}

func (p *fakePage) Alive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.closed
}

func (p *fakePage) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}

func (p *fakePage) Navigate(_ context.Context, url string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.navErr != nil {
		return p.navErr
	}
	p.url = url
	p.navigations = append(p.navigations, url)
	return nil
}

func (p *fakePage) Info(context.Context) (PageInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.infoErr != nil {
		return PageInfo{}, p.infoErr
	}
	info := p.info
	if info.URL == "" {
		info.URL = p.url
	}
	if info.Title == "" {
		info.Title = p.title
	}
	return info, nil
}

func (p *fakePage) Click(_ context.Context, t Target) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clicks = append(p.clicks, t)
	return nil
}

func (p *fakePage) Type(_ context.Context, text string, clear bool, t Target) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.typed = append(p.typed, typed{text: text, clear: clear, t: t})
	return nil
}

func (p *fakePage) Press(_ context.Context, key string, t Target) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pressed = append(p.pressed, pressed{key: key, t: t})
	return nil
}

func (p *fakePage) Screenshot(_ context.Context, dest string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.shotErr != nil {
		return p.shotErr
	}
	return os.WriteFile(dest, []byte("png"), 0o600)
}

func (p *fakePage) Reload(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reloads++
	return nil
}

func (p *fakePage) Back(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.backs++
	return nil
}

func (p *fakePage) Forward(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.forwards++
	return nil
}

func (p *fakePage) Scroll(_ context.Context, dy int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.scrolled = append(p.scrolled, dy)
	return nil
}

func (p *fakePage) Focus(context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.focused++
	return nil
}

type fakeLauncher struct {
	pages  []*fakePage
	headed []bool
}

func (f *fakeLauncher) launch(_ string, headed bool) (Page, error) {
	p := &fakePage{url: "about:blank"}
	f.pages = append(f.pages, p)
	f.headed = append(f.headed, headed)
	return p, nil
}
