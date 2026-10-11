package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"sync"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/lichdotdev/godemirror"
	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/lichdiff"
)

// diffRefresh is how often an open diff re-reads the checkout.
const diffRefresh = 2 * time.Second

// fileListWidth is the column listing the changed files.
const fileListWidth = unit.Dp(280)

// diffView shows the active checkout's working-tree diff: the changed files,
// and the one picked drawn by godemirror. It re-reads the checkout while open,
// and its buttons pull lines in and revert blocks through the backend.
type diffView struct {
	client     *lichclient.Client
	invalidate func()
	view       *godemirror.DiffView
	// shown is the file the view shows, which the buttons act on. Only the UI
	// goroutine reads it: the view calls its buttons back after the frame.
	shown string

	// Guarded by mu: what the refresh goroutine and the backend calls hand to
	// the UI goroutine, which alone touches view.
	mu       sync.Mutex
	path     string
	files    []godemirror.FilePatch
	err      error
	stop     context.CancelFunc
	refresh  chan struct{}
	answers  []pulled
	selected string

	list    widget.List
	rows    map[string]*widget.Clickable
	toSplit widget.Clickable
	toWrap  widget.Clickable
}

// pulled is the backend's answer to an Expand, waiting for the UI goroutine.
type pulled struct {
	path  string
	gap   godemirror.Gap
	lines []string
}

func newDiffView(client *lichclient.Client, invalidate func(), family string, size unit.Sp, th *material.Theme) (*diffView, error) {
	d := &diffView{client: client, invalidate: invalidate, refresh: make(chan struct{}, 1), rows: map[string]*widget.Clickable{}}
	opts, err := lichdiff.Options(th, d.expand, d.revert)
	if err != nil {
		return nil, err
	}
	view, err := godemirror.NewDiffView(family, size, opts)
	if err != nil {
		return nil, err
	}
	d.view = view
	d.list.Axis = layout.Vertical
	return d, nil
}

// watch makes the view follow path, refreshing until another path or close.
func (d *diffView) watch(path string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.path == path && d.stop != nil {
		return
	}
	if d.stop != nil {
		d.stop()
	}
	ctx, stop := context.WithCancel(context.Background())
	d.path, d.stop, d.files, d.err, d.selected = path, stop, nil, nil, ""
	go d.follow(ctx, path)
}

func (d *diffView) close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stop != nil {
		d.stop()
		d.stop = nil
	}
}

func (d *diffView) follow(ctx context.Context, path string) {
	tick := time.NewTicker(diffRefresh)
	defer tick.Stop()
	for {
		text, err := d.client.DiffText(ctx, path)
		if ctx.Err() != nil {
			return
		}
		var files []godemirror.FilePatch
		if err == nil {
			files, err = godemirror.ParsePatch(text)
		}
		if err != nil {
			log.Printf("diff %s: %v", path, err)
		}
		d.mu.Lock()
		d.files, d.err = files, err
		d.mu.Unlock()
		d.invalidate()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-d.refresh:
		}
	}
}

// expand fetches the lines of a gap off the UI goroutine.
func (d *diffView) expand(g godemirror.Gap) {
	path, rel := d.currentPath(), d.shown
	go func() {
		lines, err := d.client.FileLines(context.Background(), path, rel, "", g.From, g.To)
		if err != nil {
			log.Printf("expand %s %d..%d: %v", rel, g.From, g.To, err)
			return
		}
		d.mu.Lock()
		d.answers = append(d.answers, pulled{rel, g, lines})
		d.mu.Unlock()
		d.invalidate()
	}()
}

// revert puts a block back to HEAD, then re-reads the diff at once.
func (d *diffView) revert(lines []lichclient.RevertLine) {
	path, rel := d.currentPath(), d.shown
	go func() {
		if _, err := d.client.RevertLines(context.Background(), path, rel, lines); err != nil {
			log.Printf("revert %s: %v", rel, err)
		}
		select {
		case d.refresh <- struct{}{}:
		default:
		}
	}()
}

func (d *diffView) currentPath() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.path
}

// take hands the UI goroutine the latest files and the answers waiting.
func (d *diffView) take() ([]godemirror.FilePatch, error, []pulled) {
	d.mu.Lock()
	defer d.mu.Unlock()
	answers := d.answers
	d.answers = nil
	return d.files, d.err, answers
}

func (d *diffView) layout(gtx C, th *material.Theme) error {
	files, err, answers := d.take()
	if err != nil {
		layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
			return label(gtx, th, err.Error(), 13, colWait, font.Normal)
		})
		return nil
	}
	if len(files) == 0 {
		layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
			return label(gtx, th, "No changes in this checkout.", 13, colMuted, font.Normal)
		})
		return nil
	}
	current := d.pick(gtx, files)
	d.view.SetPatch(current)
	d.shown = current.Path
	for _, a := range answers {
		if a.path == d.shown {
			d.view.FillGap(a.gap, a.lines)
		}
	}
	var viewErr error
	layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(fileListWidth), gtx.Constraints.Max.Y))
			return d.fileList(gtx, th, files)
		}),
		layout.Rigid(func(gtx C) D {
			gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(unit.Dp(1)), gtx.Constraints.Max.Y))
			return fillRRect(gtx, colBorder, 0)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D { return d.toolbar(gtx, th, current) }),
				layout.Flexed(1, func(gtx C) D {
					gtx.Constraints.Min = gtx.Constraints.Max
					dims, err := d.view.Layout(gtx)
					viewErr = err
					return dims
				}),
			)
		}),
	)
	return viewErr
}

// pick is the file to show: the one clicked, or the first.
func (d *diffView) pick(gtx C, files []godemirror.FilePatch) godemirror.FilePatch {
	for _, f := range files {
		if d.row(f.Path).Clicked(gtx) {
			d.selected = f.Path
		}
	}
	for _, f := range files {
		if f.Path == d.selected {
			return f
		}
	}
	d.selected = files[0].Path
	return files[0]
}

func (d *diffView) row(path string) *widget.Clickable {
	c, ok := d.rows[path]
	if !ok {
		c = new(widget.Clickable)
		d.rows[path] = c
	}
	return c
}

func (d *diffView) fileList(gtx C, th *material.Theme, files []godemirror.FilePatch) D {
	return material.List(th, &d.list).Layout(gtx, len(files), func(gtx C, i int) D {
		f := files[i]
		return d.row(f.Path).Layout(gtx, func(gtx C) D {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Background{}.Layout(gtx,
				func(gtx C) D {
					if f.Path == d.selected {
						return fillRRect(gtx, colAccent, 6)
					}
					return D{Size: gtx.Constraints.Min}
				},
				func(gtx C) D {
					return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx C) D { return label(gtx, th, f.Path, 13, colForeground, font.Normal) }),
							layout.Rigid(func(gtx C) D { return label(gtx, th, fmt.Sprintf("+%d", f.Added), 12, colPass, font.Normal) }),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx C) D { return label(gtx, th, fmt.Sprintf("−%d", f.Deleted), 12, colDel, font.Normal) }),
						)
					})
				},
			)
		})
	})
}

func (d *diffView) toolbar(gtx C, th *material.Theme, f godemirror.FilePatch) D {
	if d.toSplit.Clicked(gtx) {
		d.view.Split = !d.view.Split
	}
	if d.toWrap.Clicked(gtx) {
		d.view.Wrap = !d.view.Wrap
	}
	layoutName, wrapName := "Unified", "No wrap"
	if d.view.Split {
		layoutName = "Side by side"
	}
	if d.view.Wrap {
		wrapName = "Wrap"
	}
	return layout.Inset{Left: 12, Right: 8, Top: 4, Bottom: 8}.Layout(gtx, func(gtx C) D {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx C) D { return label(gtx, th, f.Path, 13, colForeground, font.Medium) }),
			layout.Rigid(toggle(th, &d.toSplit, layoutName)),
			layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
			layout.Rigid(toggle(th, &d.toWrap, wrapName)),
		)
	})
}

// toggle is a small button naming the state it is in.
func toggle(th *material.Theme, c *widget.Clickable, text string) layout.Widget {
	return func(gtx C) D {
		return c.Layout(gtx, func(gtx C) D {
			return layout.Background{}.Layout(gtx,
				func(gtx C) D { return fillRRect(gtx, colAccent, 6) },
				func(gtx C) D {
					return layout.Inset{Top: 4, Bottom: 4, Left: 10, Right: 10}.Layout(gtx, func(gtx C) D {
						return label(gtx, th, text, 12, colForeground, font.Medium)
					})
				},
			)
		})
	}
}
