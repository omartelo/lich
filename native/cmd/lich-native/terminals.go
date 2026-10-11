package main

import (
	"context"
	"image"
	"log"
	"sync"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/gioterm"
	"github.com/omartelo/lich/native/lichclient"
)

// termView is one session's terminal: its emulator, a renderer of its own
// (renderers cache per-row state, so they cannot be shared between grids) and
// the input tag that focuses it.
type termView struct {
	id  string
	vt  *gioterm.Terminal
	r   gioterm.Renderer
	tag *int
}

// terminals keeps a termView per session opened in this window. The /ws
// stream carries every session's output; a session gets a termView, and from
// then on all its output, the first time it is shown.
type terminals struct {
	client     *lichclient.Client
	stream     *lichclient.Stream
	invalidate func()
	renderer   string
	family     string
	size       unit.Sp

	mu    sync.Mutex
	byID  map[string]*termView
	shown string
}

func (ts *terminals) onOutput(id string, data []byte) {
	ts.mu.Lock()
	t, shown := ts.byID[id], ts.shown
	ts.mu.Unlock()
	if t == nil {
		return
	}
	t.vt.Write(data)
	if id == shown {
		ts.invalidate()
	}
}

func (ts *terminals) send(id string, b []byte) {
	if len(b) == 0 {
		return
	}
	if err := ts.stream.Send(context.Background(), id, b); err != nil {
		log.Printf("send to %s: %v", id, err)
	}
}

// show returns s's terminal, attaching it on first use the way the web page
// does: the replayed tail first, then Start (a no-op for a running session),
// then the size. ponytail: output that lands between Replay and registering
// the view is dropped; the page subscribes before replaying instead.
func (ts *terminals) show(ctx context.Context, gtx layout.Context, s sessionView) (*termView, error) {
	ts.mu.Lock()
	t, previous := ts.byID[s.ID], ts.shown
	ts.shown = s.ID
	ts.mu.Unlock()
	if previous != s.ID {
		go ts.setVisible(previous, s.ID)
	}
	if t != nil {
		return t, nil
	}
	r, err := gioterm.NewRenderer(ts.renderer, ts.family, ts.size)
	if err != nil {
		return nil, err
	}
	if err := r.Measure(gtx); err != nil {
		return nil, err
	}
	cols, rows := gioterm.Fit(gtx.Constraints.Max, r)
	vt, err := gioterm.New(cols, rows, func(b []byte) { ts.send(s.ID, b) })
	if err != nil {
		return nil, err
	}
	replay, err := ts.client.Replay(ctx, s.ID)
	if err != nil {
		return nil, err
	}
	if len(replay) > 0 {
		vt.Write(replay)
	}
	t = &termView{id: s.ID, vt: vt, r: r, tag: new(int)}
	ts.mu.Lock()
	ts.byID[s.ID] = t
	ts.mu.Unlock()
	if err := ts.client.Start(ctx, s.ID, s.projectID, s.path, s.Kind, cols, rows); err != nil {
		return nil, err
	}
	return t, ts.client.Resize(ctx, s.ID, cols, rows)
}

// drop frees a closed session's terminal. Output still in flight for it finds
// no view and is discarded.
func (ts *terminals) drop(id string) {
	ts.mu.Lock()
	t := ts.byID[id]
	delete(ts.byID, id)
	if ts.shown == id {
		ts.shown = ""
	}
	ts.mu.Unlock()
	if t != nil {
		t.vt.Close()
	}
}

func (ts *terminals) setVisible(hidden, shown string) {
	ctx := context.Background()
	if hidden != "" {
		if err := ts.client.SetVisible(ctx, hidden, false); err != nil {
			log.Printf("hide %s: %v", hidden, err)
		}
	}
	if err := ts.client.SetVisible(ctx, shown, true); err != nil {
		log.Printf("show %s: %v", shown, err)
	}
}

// layout draws t into the whole of gtx.Constraints.Max, following the area's
// size with the grid and the backend PTY, and routes keys typed into it.
func (ts *terminals) layout(gtx layout.Context, t *termView) error {
	size := gtx.Constraints.Max
	defer clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops).Pop()
	if err := t.r.Measure(gtx); err != nil {
		return err
	}
	cols, rows := gioterm.Fit(size, t.r)
	if c, r := t.vt.Size(); c != cols || r != rows {
		cellW, cellH := t.r.Cell()
		if err := t.vt.Resize(cols, rows, cellW, cellH); err != nil {
			return err
		}
		go func() {
			if err := ts.client.Resize(context.Background(), t.id, cols, rows); err != nil {
				log.Printf("resize %s: %v", t.id, err)
			}
		}()
	}
	ts.input(gtx, t)
	snap, err := t.vt.Update()
	if err != nil {
		return err
	}
	if _, err := t.r.Draw(gtx, snap); err != nil {
		return err
	}
	event.Op(gtx.Ops, t.tag)
	return nil
}

func (ts *terminals) input(gtx layout.Context, t *termView) {
	_, cellH := t.r.Cell()
	filters := append(gioterm.KeyFilters(t.tag),
		pointer.Filter{Target: t.tag, Kinds: pointer.Scroll | pointer.Press, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}})
	gtx.Execute(key.FocusCmd{Tag: t.tag})
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		switch e := ev.(type) {
		case key.EditEvent:
			ts.send(t.id, []byte(e.Text))
		case key.Event:
			if e.State == key.Press {
				ts.send(t.id, t.vt.HandleKey(e))
			}
		case pointer.Event:
			if e.Kind == pointer.Scroll {
				t.vt.Scroll(int(e.Scroll.Y) / max(cellH, 1))
			}
		}
	}
}
