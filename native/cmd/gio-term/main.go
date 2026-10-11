// Spike: a terminal drawn by Gio over libghostty-vt, fed by creack/pty, to
// measure whether a native lich could render its sessions. Prints one metrics
// line per second to stderr and a summary on exit.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"os/exec"
	"runtime/pprof"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"github.com/creack/pty"
	"github.com/omartelo/lich/native/gioterm"
	"github.com/omartelo/lich/native/internal/shot"
	"golang.org/x/image/math/fixed"
)

var (
	flagFont     = flag.String("font", "JetBrainsMono Nerd Font", "font family for the grid, resolved to files by fc-match")
	flagSize     = flag.Float64("size", 13, "font size in sp")
	flagCols     = flag.Int("cols", 0, "fixed column count (0 fits the window)")
	flagWidth    = flag.Int("width", 1200, "window width in dp")
	flagH        = flag.Int("height", 800, "window height in dp")
	flagProf     = flag.String("cpuprofile", "", "write a CPU profile to this file")
	flagShot     = flag.String("shot", "", "write a PNG of the frame drawn -shot-after into the run")
	flagRenderer = flag.String("renderer", "paths", "grid renderer: paths (Gio text), atlas (glyph textures), rows (one image per row)")
	flagAfter    = flag.Duration("shot-after", 3*time.Second, "when to take -shot")
)

func main() {
	flag.Parse()
	if *flagProf != "" {
		f, err := os.Create(*flagProf)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
	}
	argv := flag.Args()
	if len(argv) == 0 {
		argv = []string{os.Getenv("SHELL")}
	}
	go func() {
		w := new(app.Window)
		w.Option(app.Title("gio-term spike"), app.Size(unit.Dp(*flagWidth), unit.Dp(*flagH)))
		code := 0
		if err := run(w, argv); err != nil {
			log.Print(err)
			code = 1
		}
		pprof.StopCPUProfile()
		os.Exit(code)
	}()
	app.Main()
}

type session struct {
	win     *app.Window
	vt      *gioterm.Terminal
	ptmx    *os.File
	exited  atomic.Bool
	bytesIn atomic.Int64
	lastIn  atomic.Int64 // unix nanos of the latest PTY read
	keyAt   int64        // unix nanos of the oldest key not yet seen on screen
}

func run(w *app.Window, argv []string) error {
	g, err := gioterm.NewRenderer(*flagRenderer, *flagFont, unit.Sp(*flagSize))
	if err != nil {
		return err
	}
	shaper := text.NewShaper()
	m := newMetrics()
	var s *session
	var ops op.Ops
	tag := new(int)
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			m.summary(s)
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			start := time.Now()
			if err := g.Measure(gtx); err != nil {
				return err
			}
			cellW, cellH := g.Cell()
			cols, rows := gioterm.Fit(gtx.Constraints.Max, g)
			if *flagCols > 0 {
				cols = *flagCols
			}
			if s == nil {
				var err error
				if s, err = startSession(w, argv, cols, rows); err != nil {
					return err
				}
			} else if c, r := s.vt.Size(); cols != c || rows != r {
				if err := s.resize(cols, rows, cellW, cellH); err != nil {
					return err
				}
			}
			s.handleInput(gtx, tag, cellH)
			snap, err := s.vt.Update()
			if err != nil {
				return err
			}
			snapDur := time.Since(start)
			repainted, err := g.Draw(gtx, snap)
			if err != nil {
				return err
			}
			event.Op(gtx.Ops, tag)
			m.drawHUD(gtx, shaper)
			drawDur := time.Since(start)
			if *flagShot != "" {
				// The window only redraws on change; make sure a frame lands
				// after the deadline even on a still screen.
				gtx.Execute(op.InvalidateCmd{At: m.started.Add(*flagAfter)})
			}
			if *flagShot != "" && time.Since(m.started) > *flagAfter {
				if err := shot.Write(*flagShot, e.Size, &ops); err != nil {
					return err
				}
				*flagShot = ""
			}
			e.Frame(gtx.Ops)
			m.frame(time.Since(start), snapDur, drawDur, repainted, s)
			if s.exited.Load() {
				m.summary(s)
				return nil
			}
		}
	}
}

// userEnv is this process's environment minus what an agent session that
// launched the spike put there, so a program in the grid sees the user's
// terminal and not a child of that session.
func userEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "CLAUDE") || strings.HasPrefix(kv, "LICH_") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func startSession(w *app.Window, argv []string, cols, rows int) (*session, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(userEnv(), "TERM=xterm-256color", "COLORTERM=truecolor")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		return nil, fmt.Errorf("start %q: %w", argv, err)
	}
	vt, err := gioterm.New(cols, rows, func(b []byte) { _, _ = ptmx.Write(b) })
	if err != nil {
		return nil, err
	}
	s := &session{win: w, vt: vt, ptmx: ptmx}
	go s.pump()
	return s, nil
}

func (s *session) pump() {
	buf := make([]byte, 64*1024)
	for {
		n, err := s.ptmx.Read(buf)
		if n > 0 {
			s.vt.Write(buf[:n])
			s.bytesIn.Add(int64(n))
			s.lastIn.Store(time.Now().UnixNano())
			s.win.Invalidate()
		}
		if err != nil {
			s.exited.Store(true)
			s.win.Invalidate()
			return
		}
	}
}

func (s *session) resize(cols, rows, cellW, cellH int) error {
	if err := pty.Setsize(s.ptmx, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}); err != nil {
		return fmt.Errorf("pty resize: %w", err)
	}
	return s.vt.Resize(cols, rows, cellW, cellH)
}

func (s *session) send(b []byte) {
	if len(b) == 0 {
		return
	}
	if s.keyAt == 0 {
		s.keyAt = time.Now().UnixNano()
	}
	if _, err := s.ptmx.Write(b); err != nil {
		log.Printf("pty write: %v", err)
	}
}

func (s *session) handleInput(gtx layout.Context, tag *int, cellH int) {
	filters := append(gioterm.KeyFilters(tag),
		pointer.Filter{Target: tag, Kinds: pointer.Scroll | pointer.Press, ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20}})
	gtx.Execute(key.FocusCmd{Tag: tag})
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		switch e := ev.(type) {
		case key.EditEvent:
			s.send([]byte(e.Text))
		case key.Event:
			if e.State == key.Press {
				s.send(s.vt.HandleKey(e))
			}
		case pointer.Event:
			if e.Kind == pointer.Scroll {
				s.vt.Scroll(int(e.Scroll.Y) / max(cellH, 1))
			}
		}
	}
}

// metrics aggregates one-second windows and keeps every window for the summary.
type metrics struct {
	started     time.Time
	totalFrames int
	windowStart time.Time
	frames      int
	frameSum    time.Duration
	frameMax    time.Duration
	snapSum     time.Duration
	drawSum     time.Duration
	rowsSum     int
	lastBytes   int64
	latencies   []time.Duration
	allFPS      []float64
	allMax      []time.Duration
	allLat      []time.Duration
	hud         string
}

func newMetrics() *metrics { return &metrics{started: time.Now(), windowStart: time.Now()} }

func (m *metrics) frame(d, snapDur, drawDur time.Duration, rows int, s *session) {
	m.frames++
	m.totalFrames++
	m.frameSum += d
	m.frameMax = max(m.frameMax, d)
	m.snapSum += snapDur
	m.drawSum += drawDur
	m.rowsSum += rows
	if s.keyAt != 0 && s.lastIn.Load() > s.keyAt {
		m.latencies = append(m.latencies, time.Duration(time.Now().UnixNano()-s.keyAt))
		s.keyAt = 0
	}
	elapsed := time.Since(m.windowStart)
	if elapsed < time.Second {
		return
	}
	bytes := s.bytesIn.Load()
	mbps := float64(bytes-m.lastBytes) / elapsed.Seconds() / (1 << 20)
	fps := float64(m.frames) / elapsed.Seconds()
	lat := "-"
	if len(m.latencies) > 0 {
		p := percentile(m.latencies, 50)
		lat = p.Round(100 * time.Microsecond).String()
		m.allLat = append(m.allLat, m.latencies...)
	}
	m.hud = fmt.Sprintf("%.0f fps  frame avg %.1fms max %.1fms  snap %.2fms  cpu %.1fms  rows/frame %.1f  in %.2f MB/s  key→frame p50 %s",
		fps, ms(m.frameSum)/float64(m.frames), ms(m.frameMax), ms(m.snapSum)/float64(m.frames), ms(m.drawSum)/float64(m.frames),
		float64(m.rowsSum)/float64(m.frames), mbps, lat)
	fmt.Fprintln(os.Stderr, m.hud)
	if m.frames > 1 {
		m.allFPS = append(m.allFPS, fps)
		m.allMax = append(m.allMax, m.frameMax)
	}
	*m = metrics{started: m.started, totalFrames: m.totalFrames, windowStart: time.Now(), lastBytes: bytes, allFPS: m.allFPS, allMax: m.allMax, allLat: m.allLat, hud: m.hud}
}

func (m *metrics) drawHUD(gtx layout.Context, shaper *text.Shaper) {
	if m.hud == "" {
		return
	}
	shaper.LayoutString(text.Parameters{PxPerEm: fixed.I(gtx.Sp(11)), MaxLines: 1, MaxWidth: 1 << 20}, m.hud)
	var gs []text.Glyph
	var width fixed.Int26_6
	for gl, ok := shaper.NextGlyph(); ok; gl, ok = shaper.NextGlyph() {
		gs = append(gs, gl)
		width = gl.X + gl.Advance
	}
	if len(gs) == 0 {
		return
	}
	pad := gtx.Dp(6)
	box := image.Rect(gtx.Constraints.Max.X-width.Ceil()-2*pad, 0, gtx.Constraints.Max.X, int(gs[0].Ascent.Ceil()+gs[0].Descent.Ceil())+2*pad)
	paint.FillShape(gtx.Ops, color.NRGBA{A: 200}, clip.Rect(box).Op())
	t := op.Offset(image.Pt(box.Min.X+pad, pad)).Push(gtx.Ops)
	shape := clip.Outline{Path: shaper.Shape(gs)}.Op().Push(gtx.Ops)
	paint.ColorOp{Color: color.NRGBA{R: 250, G: 204, B: 21, A: 255}}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	shape.Pop()
	t.Pop()
}

func (m *metrics) summary(s *session) {
	elapsed := time.Since(m.started)
	if s != nil {
		mb := float64(s.bytesIn.Load()) / (1 << 20)
		fmt.Fprintf(os.Stderr, "\nread %.1f MB in %.2fs (%.1f MB/s), %d frames (%.0f fps)\n",
			mb, elapsed.Seconds(), mb/elapsed.Seconds(), m.totalFrames, float64(m.totalFrames)/elapsed.Seconds())
	}
	if len(m.allFPS) == 0 {
		return
	}
	fps := append([]float64(nil), m.allFPS...)
	sort.Float64s(fps)
	worst := time.Duration(0)
	for _, d := range m.allMax {
		worst = max(worst, d)
	}
	fmt.Fprintf(os.Stderr, "\nsummary over %ds with frames: fps min %.0f p50 %.0f  worst frame %.1fms", len(fps), fps[0], fps[len(fps)/2], ms(worst))
	if len(m.allLat) > 0 {
		fmt.Fprintf(os.Stderr, "  key→frame p50 %s p95 %s", percentile(m.allLat, 50).Round(100*time.Microsecond), percentile(m.allLat, 95).Round(100*time.Microsecond))
	}
	fmt.Fprintln(os.Stderr)
}

func percentile(ds []time.Duration, p int) time.Duration {
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[min(len(s)*p/100, len(s)-1)]
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
