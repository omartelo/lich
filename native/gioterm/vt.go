package gioterm

/*
#cgo pkg-config: libghostty-vt-static
#include <stdlib.h>
#include <string.h>
#include <ghostty/vt.h>

extern void goWritePty(uintptr_t, uint8_t*, size_t);

static void on_write_pty(GhosttyTerminal t, void* ud, const uint8_t* data, size_t len) {
	goWritePty((uintptr_t)ud, (uint8_t*)data, len);
}

// The handle travels as the userdata pointer so each reply finds its own
// Terminal; it is an opaque integer, never dereferenced.
static GhosttyResult install_write_pty(GhosttyTerminal t, uintptr_t handle) {
	GhosttyResult r = ghostty_terminal_set(t, GHOSTTY_TERMINAL_OPT_USERDATA, (const void*)handle);
	if (r != GHOSTTY_SUCCESS) return r;
	return ghostty_terminal_set(t, GHOSTTY_TERMINAL_OPT_WRITE_PTY, (const void*)on_write_pty);
}

enum {
	CELL_HAS_FG = 1,
	CELL_HAS_BG = 2,
	CELL_BOLD = 4,
	CELL_ITALIC = 8,
	CELL_INVERSE = 16,
	CELL_FAINT = 32,
};

typedef struct {
	uint32_t cp;
	uint32_t fg;
	uint32_t bg;
	uint32_t flags;
} cell_t;

typedef struct {
	int dirty;
	uint16_t cols, rows;
	bool cursor_visible;
	uint16_t cursor_x, cursor_y;
	uint32_t bg, fg;
} snap_t;

static uint32_t pack(GhosttyColorRgb c) { return (uint32_t)c.r << 16 | (uint32_t)c.g << 8 | c.b; }

static void read_cell(GhosttyRenderStateRowCells cells, cell_t* c) {
	memset(c, 0, sizeof *c);
	uint32_t glen = 0;
	ghostty_render_state_row_cells_get(cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_GRAPHEMES_LEN, &glen);
	if (glen > 0) {
		uint32_t local[16];
		uint32_t* buf = glen <= 16 ? local : malloc(glen * sizeof(uint32_t));
		ghostty_render_state_row_cells_get(cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_GRAPHEMES_BUF, buf);
		c->cp = buf[0];
		if (buf != local) free(buf);
	}
	GhosttyColorRgb rgb;
	if (ghostty_render_state_row_cells_get(cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_FG_COLOR, &rgb) == GHOSTTY_SUCCESS) {
		c->fg = pack(rgb);
		c->flags |= CELL_HAS_FG;
	}
	if (ghostty_render_state_row_cells_get(cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_BG_COLOR, &rgb) == GHOSTTY_SUCCESS) {
		c->bg = pack(rgb);
		c->flags |= CELL_HAS_BG;
	}
	bool styled = false;
	ghostty_render_state_row_cells_get(cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_HAS_STYLING, &styled);
	if (!styled) return;
	GhosttyStyle st = GHOSTTY_INIT_SIZED(GhosttyStyle);
	ghostty_render_state_row_cells_get(cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_STYLE, &st);
	if (st.bold) c->flags |= CELL_BOLD;
	if (st.italic) c->flags |= CELL_ITALIC;
	if (st.inverse) c->flags |= CELL_INVERSE;
	if (st.faint) c->flags |= CELL_FAINT;
}

// Copies every dirty viewport row into out (cols*rows cells, row-major) and
// flags it in dirty_rows. One cgo crossing per frame instead of several per cell.
static snap_t snapshot(GhosttyRenderState rs, GhosttyTerminal t, GhosttyRenderStateRowIterator it,
		GhosttyRenderStateRowCells cells, cell_t* out, uint8_t* dirty_rows, uint16_t cols, uint16_t rows) {
	snap_t s = {0};
	if (ghostty_render_state_update(rs, t) != GHOSTTY_SUCCESS) {
		s.dirty = -1;
		return s;
	}
	GhosttyRenderStateDirty dirty = GHOSTTY_RENDER_STATE_DIRTY_FALSE;
	ghostty_render_state_get(rs, GHOSTTY_RENDER_STATE_DATA_DIRTY, &dirty);
	ghostty_render_state_get(rs, GHOSTTY_RENDER_STATE_DATA_COLS, &s.cols);
	ghostty_render_state_get(rs, GHOSTTY_RENDER_STATE_DATA_ROWS, &s.rows);
	GhosttyRenderStateCursor cur = GHOSTTY_INIT_SIZED(GhosttyRenderStateCursor);
	ghostty_render_state_get(rs, GHOSTTY_RENDER_STATE_DATA_CURSOR, &cur);
	s.cursor_visible = cur.visible && cur.viewport_has_value;
	s.cursor_x = cur.viewport_x;
	s.cursor_y = cur.viewport_y;
	GhosttyRenderStateColors colors = GHOSTTY_INIT_SIZED(GhosttyRenderStateColors);
	ghostty_render_state_get(rs, GHOSTTY_RENDER_STATE_DATA_COLORS, &colors);
	s.bg = pack(colors.background);
	s.fg = pack(colors.foreground);
	s.dirty = dirty;
	if (dirty == GHOSTTY_RENDER_STATE_DIRTY_FALSE) return s;
	if (s.cols != cols || s.rows != rows) {
		s.dirty = -2;
		return s;
	}

	ghostty_render_state_get(rs, GHOSTTY_RENDER_STATE_DATA_ROW_ITERATOR, &it);
	bool full = dirty == GHOSTTY_RENDER_STATE_DIRTY_FULL;
	uint16_t y = 0;
	for (;;) {
		if (full) {
			if (!ghostty_render_state_row_iterator_next(it)) break;
		} else if (!ghostty_render_state_row_iterator_next_dirty(it, &y)) {
			break;
		}
		if (y >= rows) break;
		ghostty_render_state_row_get(it, GHOSTTY_RENDER_STATE_ROW_DATA_CELLS, &cells);
		uint16_t x = 0;
		while (x < cols && ghostty_render_state_row_cells_next(cells)) {
			read_cell(cells, &out[(size_t)y * cols + x]);
			x++;
		}
		for (; x < cols; x++) memset(&out[(size_t)y * cols + x], 0, sizeof(cell_t));
		dirty_rows[y] = 1;
		if (full) y++;
	}
	ghostty_render_state_clean(rs);
	return s;
}

static GhosttyResult scroll_delta(GhosttyTerminal t, intptr_t delta) {
	GhosttyTerminalScrollViewport b = {0};
	b.tag = GHOSTTY_SCROLL_VIEWPORT_DELTA;
	b.value.delta = delta;
	ghostty_terminal_scroll_viewport(t, b);
	return GHOSTTY_SUCCESS;
}
*/
import "C"

import (
	"fmt"
	"runtime/cgo"
	"sync"
	"unsafe"
)

const (
	cellHasFg   = C.CELL_HAS_FG
	cellHasBg   = C.CELL_HAS_BG
	cellBold    = C.CELL_BOLD
	cellItalic  = C.CELL_ITALIC
	cellInverse = C.CELL_INVERSE
	cellFaint   = C.CELL_FAINT
)

type cell struct {
	cp, fg, bg, flags uint32
}

type snap struct {
	full          bool
	cols, rows    int
	cursorVisible bool
	cursorX       int
	cursorY       int
	bg, fg        uint32
}

//export goWritePty
func goWritePty(h C.uintptr_t, data *C.uint8_t, n C.size_t) {
	t := cgo.Handle(h).Value().(*Terminal)
	t.replies(C.GoBytes(unsafe.Pointer(data), C.int(n)))
}

// Terminal is one emulated terminal: libghostty-vt's state machine plus the
// cell grid a Renderer draws. Its methods are safe from several goroutines.
type Terminal struct {
	mu    sync.Mutex
	t     C.GhosttyTerminal
	rs    C.GhosttyRenderState
	it    C.GhosttyRenderStateRowIterator
	cells C.GhosttyRenderStateRowCells
	enc   C.GhosttyKeyEncoder
	ev    C.GhosttyKeyEvent

	handle  cgo.Handle
	replies func([]byte)

	cols, rows int
	raw        []C.cell_t
	dirty      []uint8
	grid       []cell
}

func check(what string, r C.GhosttyResult) error {
	if r != C.GHOSTTY_SUCCESS {
		return fmt.Errorf("%s: ghostty result %d", what, int(r))
	}
	return nil
}

// New creates a cols x rows terminal. replies receives what the terminal
// answers the program on its own (device attributes, mode and size reports),
// which belongs on the program's input. It runs inside Write with the
// terminal locked, so it must not call back into the Terminal.
func New(cols, rows int, replies func([]byte)) (*Terminal, error) {
	v := &Terminal{replies: replies}
	v.handle = cgo.NewHandle(v)
	steps := []struct {
		what string
		r    func() C.GhosttyResult
	}{
		{"terminal_new", func() C.GhosttyResult { return C.ghostty_terminal_new(nil, &v.t, C.uint16_t(cols), C.uint16_t(rows)) }},
		{"install_write_pty", func() C.GhosttyResult { return C.install_write_pty(v.t, C.uintptr_t(v.handle)) }},
		{"render_state_new", func() C.GhosttyResult { return C.ghostty_render_state_new(nil, &v.rs) }},
		{"row_iterator_new", func() C.GhosttyResult { return C.ghostty_render_state_row_iterator_new(nil, &v.it) }},
		{"row_cells_new", func() C.GhosttyResult { return C.ghostty_render_state_row_cells_new(nil, &v.cells) }},
		{"key_encoder_new", func() C.GhosttyResult { return C.ghostty_key_encoder_new(nil, &v.enc) }},
		{"key_event_new", func() C.GhosttyResult { return C.ghostty_key_event_new(nil, &v.ev) }},
	}
	for _, s := range steps {
		if err := check(s.what, s.r()); err != nil {
			return nil, err
		}
	}
	v.allocGrid(cols, rows)
	return v, nil
}

func (v *Terminal) allocGrid(cols, rows int) {
	v.cols, v.rows = cols, rows
	v.raw = make([]C.cell_t, cols*rows)
	v.dirty = make([]uint8, rows)
	v.grid = make([]cell, cols*rows)
}

// Write feeds program output to the terminal.
func (v *Terminal) Write(b []byte) {
	v.mu.Lock()
	C.ghostty_terminal_vt_write(v.t, (*C.uint8_t)(unsafe.Pointer(&b[0])), C.size_t(len(b)))
	v.mu.Unlock()
}

// Resize changes the grid; cellW and cellH are what size reports tell the
// program a cell measures in pixels.
func (v *Terminal) Resize(cols, rows, cellW, cellH int) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := check("terminal_resize", C.ghostty_terminal_resize(v.t, C.uint16_t(cols), C.uint16_t(rows), C.uint32_t(cellW), C.uint32_t(cellH))); err != nil {
		return err
	}
	v.allocGrid(cols, rows)
	return nil
}

// Scroll moves the viewport through scrollback, negative is up.
func (v *Terminal) Scroll(delta int) {
	v.mu.Lock()
	C.scroll_delta(v.t, C.intptr_t(delta))
	v.mu.Unlock()
}

// Size is the grid in cells.
func (v *Terminal) Size() (cols, rows int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.cols, v.rows
}

// Snapshot is the terminal state a Renderer draws for one frame.
type Snapshot struct {
	s     snap
	cells []cell
	dirty []uint8
}

// Update takes the frame's Snapshot. It belongs to the frame goroutine: the
// Snapshot shares its cells with the Terminal until the next Update.
func (v *Terminal) Update() (Snapshot, error) {
	s, dirty, err := v.update()
	return Snapshot{s: s, cells: v.grid, dirty: dirty}, err
}

// Close frees the terminal. It must not be used afterwards.
func (v *Terminal) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	C.ghostty_key_event_free(v.ev)
	C.ghostty_key_encoder_free(v.enc)
	C.ghostty_render_state_row_cells_free(v.cells)
	C.ghostty_render_state_row_iterator_free(v.it)
	C.ghostty_render_state_free(v.rs)
	C.ghostty_terminal_free(v.t)
	v.handle.Delete()
}

// update refreshes grid from the terminal and returns which rows changed.
func (v *Terminal) update() (snap, []uint8, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	clear(v.dirty)
	s := C.snapshot(v.rs, v.t, v.it, v.cells, &v.raw[0], (*C.uint8_t)(&v.dirty[0]), C.uint16_t(v.cols), C.uint16_t(v.rows))
	if s.dirty == -1 {
		return snap{}, nil, fmt.Errorf("render_state_update failed")
	}
	if s.dirty == -2 {
		return snap{}, nil, fmt.Errorf("render state is %dx%d, grid is %dx%d", s.cols, s.rows, v.cols, v.rows)
	}
	for y, d := range v.dirty {
		if d == 0 {
			continue
		}
		for x := range v.cols {
			r := v.raw[y*v.cols+x]
			v.grid[y*v.cols+x] = cell{cp: uint32(r.cp), fg: uint32(r.fg), bg: uint32(r.bg), flags: uint32(r.flags)}
		}
	}
	return snap{
		full:          s.dirty == C.GHOSTTY_RENDER_STATE_DIRTY_FULL,
		cols:          int(s.cols),
		rows:          int(s.rows),
		cursorVisible: bool(s.cursor_visible),
		cursorX:       int(s.cursor_x),
		cursorY:       int(s.cursor_y),
		bg:            uint32(s.bg),
		fg:            uint32(s.fg),
	}, v.dirty, nil
}

// Key names libghostty's encoder takes for the special keys the spike maps.
const (
	keyEnter     = C.GHOSTTY_KEY_ENTER
	keyBackspace = C.GHOSTTY_KEY_BACKSPACE
	keyTab       = C.GHOSTTY_KEY_TAB
	keyEscape    = C.GHOSTTY_KEY_ESCAPE
	keyUp        = C.GHOSTTY_KEY_ARROW_UP
	keyDown      = C.GHOSTTY_KEY_ARROW_DOWN
	keyLeft      = C.GHOSTTY_KEY_ARROW_LEFT
	keyRight     = C.GHOSTTY_KEY_ARROW_RIGHT
	keyHome      = C.GHOSTTY_KEY_HOME
	keyEnd       = C.GHOSTTY_KEY_END
	keyPageUp    = C.GHOSTTY_KEY_PAGE_UP
	keyPageDown  = C.GHOSTTY_KEY_PAGE_DOWN
	keyDelete    = C.GHOSTTY_KEY_DELETE
	keyA         = C.GHOSTTY_KEY_A
	keyDigit0    = C.GHOSTTY_KEY_DIGIT_0
	keyF1        = C.GHOSTTY_KEY_F1
	keySpace     = C.GHOSTTY_KEY_SPACE
	keyUnknown   = C.GHOSTTY_KEY_UNIDENTIFIED
	keyBackquote = C.GHOSTTY_KEY_BACKQUOTE
	keyBackslash = C.GHOSTTY_KEY_BACKSLASH
	keyBracketL  = C.GHOSTTY_KEY_BRACKET_LEFT
	keyBracketR  = C.GHOSTTY_KEY_BRACKET_RIGHT
	keyComma     = C.GHOSTTY_KEY_COMMA
	keyEqual     = C.GHOSTTY_KEY_EQUAL
	keyMinus     = C.GHOSTTY_KEY_MINUS
	keyPeriod    = C.GHOSTTY_KEY_PERIOD
	keyQuote     = C.GHOSTTY_KEY_QUOTE
	keySemicolon = C.GHOSTTY_KEY_SEMICOLON
	keySlash     = C.GHOSTTY_KEY_SLASH
	modShift     = C.GHOSTTY_MODS_SHIFT
	modCtrl      = C.GHOSTTY_MODS_CTRL
	modAlt       = C.GHOSTTY_MODS_ALT
)

// encodeKey turns a key press into the bytes the program expects, honouring
// the terminal's current modes (cursor keys, kitty flags). The kitty encoding
// names a key by its unshifted codepoint and emits nothing without one, so a
// character key must pass it, with text as what the key types; special keys
// pass 0 and "".
func (v *Terminal) encodeKey(key int, mods int, unshifted rune, text string) []byte {
	v.mu.Lock()
	defer v.mu.Unlock()
	C.ghostty_key_encoder_setopt_from_terminal(v.enc, v.t)
	C.ghostty_key_event_set_action(v.ev, C.GHOSTTY_KEY_ACTION_PRESS)
	C.ghostty_key_event_set_key(v.ev, C.GhosttyKey(key))
	C.ghostty_key_event_set_mods(v.ev, C.GhosttyMods(mods))
	C.ghostty_key_event_set_unshifted_codepoint(v.ev, C.uint32_t(unshifted))
	cs := C.CString(text)
	defer C.free(unsafe.Pointer(cs))
	C.ghostty_key_event_set_utf8(v.ev, cs, C.size_t(len(text)))
	var buf [128]C.char
	var n C.size_t
	if C.ghostty_key_encoder_encode(v.enc, v.ev, &buf[0], C.size_t(len(buf)), &n) != C.GHOSTTY_SUCCESS {
		return nil
	}
	return C.GoBytes(unsafe.Pointer(&buf[0]), C.int(n))
}
