package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/omartelo/lich/internal/chromium"
)

// chromeLaunch starts a Chromium-family browser via CDP. ExecPath is
// chromium.FindBrowser (system Chrome/Edge/… — never lich-shell); UserDataDir
// is this ephemeral dir, never lich/chromium-profile. Attaching to the UI
// window would expose the page and the session token; this process must stay
// a sidecar.
func chromeLaunch(dir string, headed bool) (Page, error) {
	exe, err := chromium.FindBrowser(exec.LookPath)
	if err != nil {
		return nil, err
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts, chromedp.ExecPath(exe), chromedp.UserDataDir(dir))
	if headed {
		opts = append(opts, chromedp.Flag("headless", false))
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(ctx); err != nil {
		cancel()
		allocCancel()
		return nil, err
	}
	return &chromePage{ctx: ctx, cancel: cancel, allocCancel: allocCancel}, nil
}

type chromePage struct {
	ctx         context.Context
	cancel      context.CancelFunc
	allocCancel context.CancelFunc
}

func (p *chromePage) Alive() bool { return p.ctx.Err() == nil }

func (p *chromePage) Close() error {
	p.cancel()
	p.allocCancel()
	return nil
}

// run drives CDP on the allocator context chromedp attached, applying wait's
// deadline. Service callers pass a timeout derived from Background; chromedp.Run
// on that context is "invalid context" and the headed window is then closed.
func (p *chromePage) run(wait context.Context, actions ...chromedp.Action) error {
	ctx, cancel := withPageDeadline(p.ctx, wait)
	defer cancel()
	return chromedp.Run(ctx, actions...)
}

func (p *chromePage) Navigate(ctx context.Context, url string) error {
	return p.run(ctx, chromedp.Navigate(url))
}

func (p *chromePage) Info(ctx context.Context) (PageInfo, error) {
	var info PageInfo
	if err := p.run(ctx, chromedp.Evaluate(pageInfoScript, &info)); err != nil {
		return PageInfo{}, err
	}
	return info, nil
}

func (p *chromePage) Click(ctx context.Context, t Target) error {
	pt, err := p.resolve(ctx, t)
	if err != nil {
		return err
	}
	return p.run(ctx, chromedp.MouseClickXY(pt.X, pt.Y))
}

func (p *chromePage) Type(ctx context.Context, text string, clear bool, t Target) error {
	if !t.empty() {
		if err := p.Click(ctx, t); err != nil {
			return err
		}
	}
	if clear {
		err := p.run(ctx, chromedp.Evaluate(`(() => {
			const el = document.activeElement;
			if (el && 'value' in el) el.value = '';
		})()`, nil))
		if err != nil {
			return err
		}
	}
	return p.run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return input.InsertText(text).Do(ctx)
	}))
}

func (p *chromePage) Press(ctx context.Context, key string, t Target) error {
	seq, err := keySequence(key)
	if err != nil {
		return err
	}
	if !t.empty() {
		if err := p.Click(ctx, t); err != nil {
			return err
		}
	}
	return p.run(ctx, chromedp.KeyEvent(seq))
}

func (p *chromePage) Screenshot(ctx context.Context, dest string) error {
	var buf []byte
	if err := p.run(ctx, chromedp.FullScreenshot(&buf, 90)); err != nil {
		return err
	}
	return os.WriteFile(dest, buf, 0o600)
}

func (p *chromePage) Reload(ctx context.Context) error {
	return p.run(ctx, chromedp.Reload())
}

func (p *chromePage) Back(ctx context.Context) error {
	return p.run(ctx, chromedp.NavigateBack())
}

func (p *chromePage) Forward(ctx context.Context) error {
	return p.run(ctx, chromedp.NavigateForward())
}

func (p *chromePage) Scroll(ctx context.Context, dy int) error {
	return p.run(ctx, chromedp.Evaluate(fmt.Sprintf("window.scrollBy(0, %d)", dy), nil))
}

func (p *chromePage) Focus(ctx context.Context) error {
	return p.run(ctx, chromedp.Evaluate("window.focus()", nil))
}

// withPageDeadline hangs wait's timeout off the chromedp page context. A
// timeout from context.Background has no CDP target; wrapping it the other
// way around is how OpenVisible returned "invalid context" and closed the
// window it had just opened.
func withPageDeadline(pageCtx, waitCtx context.Context) (context.Context, context.CancelFunc) {
	timeout := pageWait
	if waitCtx != nil {
		if d, ok := waitCtx.Deadline(); ok {
			timeout = time.Until(d)
			if timeout < 0 {
				timeout = 0
			}
		}
	}
	return context.WithTimeout(pageCtx, timeout)
}

type point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

const resolveJS = `(function(index, selector, x, y) {
  if (typeof x === 'number' && typeof y === 'number') {
    return {x: x, y: y};
  }
  var el = null;
  if (selector) el = document.querySelector(selector);
  else if (typeof index === 'number') {
    el = document.querySelector('[data-lich-idx="' + index + '"]');
  }
  if (!el) return null;
  var r = el.getBoundingClientRect();
  return {x: r.left + r.width / 2, y: r.top + r.height / 2};
})`

func (p *chromePage) resolve(ctx context.Context, t Target) (point, error) {
	args, err := json.Marshal([]any{t.Index, t.Selector, t.X, t.Y})
	if err != nil {
		return point{}, err
	}
	expr := resolveJS + "(" + string(args[1:len(args)-1]) + ")"
	var pt *point
	if err := p.run(ctx, chromedp.Evaluate(expr, &pt)); err != nil {
		return point{}, err
	}
	if pt == nil {
		return point{}, fmt.Errorf("no element matches that target")
	}
	return *pt, nil
}
