package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/omartelo/lich/native/ui/icons"
)

// MenuEntryKind is the shadcn menu part an entry draws.
type MenuEntryKind uint8

const (
	MenuItem MenuEntryKind = iota
	MenuCheckboxItem
	MenuRadioItem
	MenuSubTrigger
	MenuLabel
	MenuSeparator
	// MenuSwatch is a color swatch cell of a grid submenu, as
	// CardColorMenu.tsx draws one; Label is its Hint.
	MenuSwatch
)

// MenuEntry is one row of a DropdownMenu or ContextMenu, the props of its
// TSX part. Variant is VariantDefault or VariantDestructive.
type MenuEntry struct {
	Kind     MenuEntryKind
	Label    string
	Icon     *icons.Icon
	Shortcut string
	Variant  Variant
	Inset    bool
	Disabled bool
	// Checked is the state a checkbox or radio item draws, or the current
	// swatch's outline.
	Checked bool
	// Sub is the submenu a MenuSubTrigger opens.
	Sub []MenuEntry
	// Columns lays a MenuSubTrigger's Sub out as a min-w-0 grid this many
	// cells wide; 0 is the usual column of rows.
	Columns int
	// Swatch is a MenuSwatch's color; the zero color is the theme option,
	// half accent and half background.
	Swatch color.NRGBA
	// OnClick runs when an item or radio item is chosen; an item closes the
	// menu first, a radio item keeps it open, as base-ui does.
	OnClick func()
	// OnCheckedChange runs with the new state when a checkbox item is
	// chosen; the menu stays open.
	OnCheckedChange func(checked bool)
}

func (e MenuEntry) actionable() bool {
	return !e.Disabled && e.Kind != MenuLabel && e.Kind != MenuSeparator
}

// MenuState is a menu's open state, owned by the caller like a
// widget.Clickable.
type MenuState struct {
	open     bool
	focus    bool
	origin   image.Point
	anchor   image.Rectangle
	levels   []*menuLevel
	trigger  struct{ _ byte }
	backdrop popupBackdrop
}

// menuLevel is one open panel: the root menu, then each open submenu.
type menuLevel struct {
	highlight int
	pressed   bool
	rows      []image.Rectangle
	// hints are a grid's per-cell tooltips.
	hints []Tooltip
	// shown is when the panel first drew, the start of its animate-in.
	shown time.Time
}

// Opened reports whether the menu is showing.
func (s *MenuState) Opened() bool { return s.open }

func (s *MenuState) menuOpen(anchor image.Rectangle, origin image.Point) {
	s.open, s.focus, s.anchor, s.origin = true, true, anchor, origin
	s.levels = []*menuLevel{{highlight: -1}}
}

// menuPrune drops the highlights and submenus that entries rebuilt since the
// last frame no longer have.
func (s *MenuState) menuPrune(entries []MenuEntry) {
	for k, lv := range s.levels {
		if lv.highlight >= len(entries) {
			lv.highlight = -1
		}
		if k+1 == len(s.levels) {
			return
		}
		if lv.highlight < 0 || entries[lv.highlight].Kind != MenuSubTrigger {
			s.levels = s.levels[:k+1]
			return
		}
		entries = entries[lv.highlight].Sub
	}
}

// menuGeometry is what differs between dropdown-menu.tsx and
// context-menu.tsx; every other class is shared.
type menuGeometry struct {
	minWidth, subMinWidth          unit.Dp
	itemPadX, itemPadY, shortcutPL unit.Dp
	root, sub                      popupPlace
}

const (
	menuLineSM          unit.Dp = 20 // text-sm's line-height, 1.25rem
	menuLineXS          unit.Dp = 16 // text-xs's, 1rem
	menuRingOpacity             = 0.1
	menuDestructiveFill         = 0.2 // dark:focus:bg-destructive/20
)

var (
	menuPanelPad       = Space(1)
	menuGap            = Space(2)
	menuIconSize       = Space(4)
	menuInsetPL        = Space(8)
	menuIndicatorRight = Space(2)
	menuChevron        = icons.Lucide("chevron-right")
	menuCheck          = icons.Lucide("check")
)

// box is a row's padding: py-1.5 px-2 unless the part's classes say
// otherwise, data-inset:pl-8 on top.
func (g *menuGeometry) box(e MenuEntry) (padL, padR, padY unit.Dp) {
	padL, padR, padY = Space(2), Space(2), Space(1.5)
	switch e.Kind {
	case MenuItem:
		padL, padR, padY = g.itemPadX, g.itemPadX, g.itemPadY
	case MenuCheckboxItem, MenuRadioItem:
		padR = Space(8)
	}
	if e.Inset {
		padL = menuInsetPL
	}
	return padL, padR, padY
}

// menuCore is the behaviour and drawing both menus share; they differ only in
// how they open and in their geometry.
type menuCore struct {
	th      *Theme
	root    *Root
	s       *MenuState
	entries []MenuEntry
	geo     *menuGeometry
	// panel is th on the popover, for what the panels hold.
	panel *Theme
}

// menuLayoutTrigger lays out the trigger under an area of its own, the parent
// of the trigger's handlers, so the menu sees presses the trigger's own
// Clickable also takes.
func menuLayoutTrigger(gtx C, tag event.Tag, w layout.Widget) D {
	macro := op.Record(gtx.Ops)
	dims := w(gtx)
	call := macro.Stop()
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tag)
	call.Add(gtx.Ops)
	return dims
}

// menuTriggerPress returns where, in the trigger's coordinates, the last
// press of button landed this frame.
func menuTriggerPress(gtx C, tag event.Tag, button pointer.Buttons) (f32.Point, bool) {
	var at f32.Point
	pressed := false
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press})
		if !ok {
			return at, pressed
		}
		if pe, ok := ev.(pointer.Event); ok && pe.Buttons.Contain(button) {
			at, pressed = pe.Position, true
		}
	}
}

func (m menuCore) layout(gtx C) {
	if !m.s.open {
		return
	}
	m.s.menuPrune(m.entries)
	if m.s.backdrop.pressed(gtx) {
		m.close(gtx)
	}
	for k := 0; m.s.open && k < len(m.s.levels); k++ {
		m.panelEvents(gtx, k)
	}
	m.keyEvents(gtx)
	if !m.s.open {
		return
	}
	m.panel = m.th.On(m.th.Popover)
	macro := op.Record(gtx.Ops)
	m.drawOverlay(gtx)
	op.Defer(gtx.Ops, macro.Stop())
}

func (m menuCore) close(gtx C) {
	if gtx.Focused(&m.s.backdrop) {
		gtx.Execute(key.FocusCmd{})
	}
	m.s.open, m.s.levels = false, nil
}

// levelEntries walks the open submenus down to level k.
func (m menuCore) levelEntries(k int) []MenuEntry {
	entries := m.entries
	for _, lv := range m.s.levels[:k] {
		entries = entries[lv.highlight].Sub
	}
	return entries
}

func (m menuCore) panelEvents(gtx C, k int) {
	lv := m.s.levels[k]
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: lv, Kinds: pointer.Move | pointer.Enter | pointer.Leave | pointer.Press | pointer.Release})
		if !ok {
			return
		}
		pe, ok := ev.(pointer.Event)
		if ok {
			// The deferred panels are outside the root's area, so it never
			// sees these events; a panel's are in window coordinates already,
			// what the root would have recorded for a tooltip inside it.
			m.root.pointer = pe.Position
		}
		live := m.s.open && k < len(m.s.levels) && m.s.levels[k] == lv
		if ok && live {
			m.panelPointer(gtx, k, pe)
		}
	}
}

func (m menuCore) panelPointer(gtx C, k int, pe pointer.Event) {
	lv, entries := m.s.levels[k], m.levelEntries(k)
	i := menuRowAt(lv.rows, pe.Position.Round())
	hit := i >= 0 && i < len(entries) && entries[i].actionable()
	switch pe.Kind {
	case pointer.Move, pointer.Enter:
		if hit && lv.highlight != i {
			m.highlight(k, i)
			m.openSub(k, entries[i], false)
		}
	case pointer.Leave:
		if len(m.s.levels) == k+1 {
			lv.highlight = -1
		}
	case pointer.Press:
		lv.pressed = true
	case pointer.Release:
		if lv.pressed && hit {
			m.activate(gtx, k, i, false)
		}
		lv.pressed = false
	}
}

func menuRowAt(rows []image.Rectangle, pt image.Point) int {
	for i, r := range rows {
		if pt.In(r) {
			return i
		}
	}
	return -1
}

// highlight moves level k's highlight to row i and closes any submenu below.
func (m menuCore) highlight(k, i int) {
	m.s.levels[k].highlight = i
	m.s.levels = m.s.levels[:k+1]
}

// openSub opens e's submenu under level k; from the keyboard its first item
// is highlighted, as base-ui does on ArrowRight.
func (m menuCore) openSub(k int, e MenuEntry, keyboard bool) {
	if e.Kind != MenuSubTrigger || len(m.s.levels) > k+1 {
		return
	}
	first := -1
	if keyboard {
		first = menuStep(e.Sub, -1, 1)
	}
	m.s.levels = append(m.s.levels, &menuLevel{highlight: first})
}

func (m menuCore) activate(gtx C, k, i int, keyboard bool) {
	e := m.levelEntries(k)[i]
	switch e.Kind {
	case MenuSubTrigger:
		m.highlight(k, i)
		m.openSub(k, e, keyboard)
	case MenuCheckboxItem:
		if e.OnCheckedChange != nil {
			e.OnCheckedChange(!e.Checked)
		}
	case MenuRadioItem:
		if e.OnClick != nil {
			e.OnClick()
		}
	default:
		m.close(gtx)
		if e.OnClick != nil {
			e.OnClick()
		}
	}
}

// menuStep is the next actionable row from `from` in direction dir, looping
// as base-ui's loopFocus does; from -1 or len(entries) starts at an end.
func menuStep(entries []MenuEntry, from, dir int) int {
	n := len(entries)
	if from < 0 && dir < 0 {
		from = n
	}
	for j := 1; j <= n; j++ {
		i := ((from+dir*j)%n + n) % n
		if entries[i].actionable() {
			return i
		}
	}
	return -1
}

var menuKeys = []key.Name{
	key.NameUpArrow, key.NameDownArrow, key.NameLeftArrow, key.NameRightArrow,
	key.NameHome, key.NameEnd, key.NameReturn, key.NameEnter, key.NameSpace, key.NameEscape,
}

// keyEvents reads the keys on the backdrop: it is the one handler laid out
// under the whole menu for as long as it is open, so it holds the focus.
func (m menuCore) keyEvents(gtx C) {
	filters := []event.Filter{key.FocusFilter{Target: &m.s.backdrop}}
	for _, name := range menuKeys {
		filters = append(filters, key.Filter{Focus: &m.s.backdrop, Name: name})
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		if ke, ok := ev.(key.Event); ok && ke.State == key.Press && m.s.open {
			m.key(gtx, ke.Name)
		}
	}
}

// keyLevel is the panel the keyboard drives: the deepest one with a
// highlight, so a submenu opened by hover leaves the keys in its parent until
// ArrowRight moves into it.
func (m menuCore) keyLevel() int {
	for k := len(m.s.levels) - 1; k > 0; k-- {
		if m.s.levels[k].highlight >= 0 {
			return k
		}
	}
	return 0
}

func (m menuCore) key(gtx C, name key.Name) {
	k := m.keyLevel()
	if cols := m.levelColumns(k); cols > 0 && m.gridKey(k, cols, name) {
		return
	}
	lv, entries := m.s.levels[k], m.levelEntries(k)
	h := lv.highlight
	switch name {
	case key.NameEscape:
		m.close(gtx)
	case key.NameDownArrow:
		m.highlight(k, menuStep(entries, h, 1))
	case key.NameUpArrow:
		m.highlight(k, menuStep(entries, h, -1))
	case key.NameHome:
		m.highlight(k, menuStep(entries, -1, 1))
	case key.NameEnd:
		m.highlight(k, menuStep(entries, len(entries), -1))
	case key.NameRightArrow:
		if h >= 0 {
			m.openSub(k, entries[h], true)
		}
	case key.NameLeftArrow:
		if k > 0 {
			m.s.levels = m.s.levels[:k]
		}
	case key.NameReturn, key.NameEnter, key.NameSpace:
		if h >= 0 {
			m.activate(gtx, k, h, true)
		}
	}
}

// drawOverlay runs deferred, under the trigger's transform: it moves back to
// window coordinates, lays the backdrop over the whole window (the menu is
// modal, as base-ui's is), then each open panel beside the row that opened it.
func (m menuCore) drawOverlay(gtx C) {
	defer op.Offset(m.s.origin.Mul(-1)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Constraints{Max: m.root.window}
	m.s.backdrop.layout(gtx, m.root.window)
	if m.s.focus {
		gtx.Execute(key.FocusCmd{Tag: &m.s.backdrop})
		m.s.focus = false
	}
	anchor, place, minWidth, entries, cols := m.s.anchor, m.geo.root, m.geo.minWidth, m.entries, 0
	for _, lv := range m.s.levels {
		m.drawPanel(gtx, lv, anchor, place, func(gtx C) image.Point {
			if cols > 0 {
				return m.layoutGrid(gtx, lv, entries, cols)
			}
			return m.layoutPanel(gtx, lv, entries, minWidth)
		})
		if lv.highlight < 0 || lv.highlight >= len(entries) {
			return
		}
		trigger := entries[lv.highlight]
		anchor, place, minWidth, entries, cols = lv.rows[lv.highlight], m.geo.sub, m.geo.subMinWidth, trigger.Sub, trigger.Columns
	}
}

// drawPanel places what layout draws at its origin beside anchor.
func (m menuCore) drawPanel(gtx C, lv *menuLevel, anchor image.Rectangle, place popupPlace, layout func(C) image.Point) {
	macro := op.Record(gtx.Ops)
	size := layout(gtx)
	call := macro.Stop()
	box, side := place.box(gtx, anchor, size, image.Rectangle{Max: m.root.window})
	pos := box.Min
	for i := range lv.rows {
		lv.rows[i] = lv.rows[i].Add(pos)
	}
	area := clip.Rect{Min: pos, Max: pos.Add(size)}.Push(gtx.Ops)
	event.Op(gtx.Ops, lv)
	area.Pop()
	defer op.Offset(pos).Push(gtx.Ops).Pop()
	defer menuEnter(gtx, lv, side, anchor.Min.Sub(pos), size)()
	call.Add(gtx.Ops)
}

// menuDuration is the content's duration-100.
const menuDuration = 100 * time.Millisecond

// menuEnter starts a panel's animate-in: fade-in-0, zoom-in-95 from the
// anchor (origin-(--transform-origin)) and slide-in-2 from the anchor's
// side, over menuDuration from its first frame. Hit testing stays on the
// final box. The returned func pops what it pushed.
//
// ponytail: no animate-out; a closed menu is gone at once. Adding it means
// drawing the last entries for menuDuration after close.
func menuEnter(gtx C, lv *menuLevel, side Side, anchor, size image.Point) func() {
	if lv.shown.IsZero() {
		lv.shown = gtx.Now
	}
	elapsed := gtx.Now.Sub(lv.shown)
	if elapsed >= menuDuration {
		return func() {}
	}
	gtx.Execute(op.InvalidateCmd{})
	p := ease(float32(elapsed) / float32(menuDuration))
	origin, toward := menuOrigin(side, anchor, size)
	slide := toward.Mul(float32(gtx.Dp(Space(2))) * (1 - p))
	scale := zoomFrom + (1-zoomFrom)*p
	transform := op.Affine(f32.Affine2D{}.Scale(origin, f32.Pt(scale, scale)).Offset(slide)).Push(gtx.Ops)
	opacity := paint.PushOpacity(gtx.Ops, p)
	return func() {
		opacity.Pop()
		transform.Pop()
	}
}

// menuOrigin is base-ui's transform origin for a panel placed on side of an
// anchor at the panel-local point anchor: the panel edge facing it, at the
// anchor's position along that edge. toward points back at the anchor.
func menuOrigin(side Side, anchor, size image.Point) (origin, toward f32.Point) {
	x := float32(min(max(anchor.X, 0), size.X))
	y := float32(min(max(anchor.Y, 0), size.Y))
	switch side {
	case SideRight:
		return f32.Pt(0, y), f32.Pt(-1, 0)
	case SideLeft:
		return f32.Pt(float32(size.X), y), f32.Pt(1, 0)
	case SideTop:
		return f32.Pt(x, float32(size.Y)), f32.Pt(0, 1)
	}
	return f32.Pt(x, 0), f32.Pt(0, -1)
}

// layoutPanel draws the popup at its origin and records each row's rectangle
// in lv.rows. The panel is as wide as its widest row (w-auto), never
// narrower than minWidth.
func (m menuCore) layoutPanel(gtx C, lv *menuLevel, entries []MenuEntry, minWidth unit.Dp) image.Point {
	pad := gtx.Dp(menuPanelPad)
	width, height := gtx.Dp(minWidth)-2*pad, 2*pad
	rows := make([]menuRow, len(entries))
	for i, e := range entries {
		rows[i] = m.measureRow(gtx, e, i == lv.highlight)
		width, height = max(width, rows[i].natural), height+rows[i].height
	}
	size := image.Pt(width+2*pad, height)
	m.drawSurface(gtx, size)
	lv.rows = lv.rows[:0]
	y := pad
	for _, r := range rows {
		rect := image.Rect(pad, y, pad+width, y+r.height)
		lv.rows = append(lv.rows, rect)
		r.draw(gtx, rect)
		y += r.height
	}
	return size
}

// drawSurface is rounded-md bg-popover ring-1 ring-foreground/10. The ring
// is a 2px stroke on the edge whose inner half the fill covers, leaving 1px
// outside the box as a box-shadow ring does, over the surface under the menu.
// ponytail: no shadow-md; a 10% black shadow barely reads on the dark theme.
func (m menuCore) drawSurface(gtx C, size image.Point) {
	rr := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(RadiusMD))
	ring := clip.Stroke{Path: rr.Path(gtx.Ops), Width: float32(2 * gtx.Dp(1))}.Op()
	paint.FillShape(gtx.Ops, m.th.Over(Alpha(m.th.Foreground, menuRingOpacity)), ring)
	paint.FillShape(gtx.Ops, m.th.Popover, rr.Op(gtx.Ops))
}

// menuRow is one measured row: its leading group (icon, label) and trailing
// group (shortcut, chevron or check), recorded to draw once the panel's
// width is known.
type menuRow struct {
	left, right            op.CallOp
	leftW, rightW          int
	padL, padY, rightInset int
	height, natural        int
	bg                     color.NRGBA
	separator              bool
}

type menuColors struct{ bg, fg, icon, shortcut color.NRGBA }

// rowColors are the dark-mode classes of a row, at rest or focused, with
// the translucent ones composited onto the popover.
func (m menuCore) rowColors(e MenuEntry, highlighted bool) menuColors {
	p := m.panel.Palette
	c := menuColors{fg: p.PopoverForeground, icon: p.PopoverForeground, shortcut: p.MutedForeground}
	if e.Kind == MenuLabel {
		c.fg = p.MutedForeground
	}
	if highlighted {
		c = menuColors{bg: p.Accent, fg: p.AccentForeground, icon: p.AccentForeground, shortcut: p.AccentForeground}
	}
	if e.Variant == VariantDestructive {
		c.fg, c.icon = p.Destructive, p.Destructive
		if highlighted {
			c.bg = m.panel.Over(Alpha(p.Destructive, menuDestructiveFill))
		}
	}
	if e.Disabled {
		faded := func(c color.NRGBA) color.NRGBA { return m.panel.Over(Alpha(c, disabledOpacity)) }
		c = menuColors{fg: faded(c.fg), icon: faded(c.icon), shortcut: faded(c.shortcut)}
	}
	return c
}

func (m menuCore) measureRow(gtx C, e MenuEntry, highlighted bool) menuRow {
	if e.Kind == MenuSeparator {
		inset := gtx.Dp(Space(1)) // mx-1 my-1
		return menuRow{bg: m.panel.Over(m.panel.Border), padL: inset, padY: inset, height: gtx.Dp(1) + 2*inset, separator: true}
	}
	padL, padR, padY := m.geo.box(e)
	col := m.rowColors(e, highlighted)
	line := menuLineSM
	if e.Kind == MenuLabel {
		line = menuLineXS
	}
	lineGtx := gtx
	lineGtx.Constraints = layout.Constraints{Min: image.Pt(0, gtx.Dp(line)), Max: image.Pt(gtx.Constraints.Max.X, gtx.Dp(line))}
	row := menuRow{bg: col.bg, padL: gtx.Dp(padL), padY: gtx.Dp(padY)}
	row.left, row.leftW = menuRecord(lineGtx, func(gtx C) D { return m.leading(gtx, e, col) })
	row.right, row.rightW = menuRecord(lineGtx, func(gtx C) D { return m.trailing(gtx, e, col) })
	row.height = 2*row.padY + gtx.Dp(line)
	var trail int
	row.rightInset, trail = m.trailRoom(gtx, e, row.rightW, padR)
	row.natural = row.padL + row.leftW + trail
	return row
}

func menuRecord(gtx C, w layout.Widget) (op.CallOp, int) {
	macro := op.Record(gtx.Ops)
	d := w(gtx)
	return macro.Stop(), d.Size.X
}

// trailRoom is the room right of the label: the ml-auto group after its
// gap (and a context menu shortcut's pl-5), or the pr-8 a checkbox's
// absolute indicator sits in.
func (m menuCore) trailRoom(gtx C, e MenuEntry, rightW int, padR unit.Dp) (inset, trail int) {
	inset = gtx.Dp(padR)
	switch {
	case e.Kind == MenuCheckboxItem || e.Kind == MenuRadioItem:
		return gtx.Dp(menuIndicatorRight), inset
	case rightW > 0:
		gap := gtx.Dp(menuGap)
		if e.Kind == MenuItem {
			gap += gtx.Dp(m.geo.shortcutPL)
		}
		return inset, gap + rightW + inset
	}
	return inset, inset
}

func (m menuCore) leading(gtx C, e MenuEntry, col menuColors) D {
	if e.Kind == MenuLabel {
		return layout.W.Layout(gtx, m.panel.Text(TextXS, e.Label).In(col.fg).Weight(font.Medium).Layout)
	}
	var children []layout.FlexChild
	if e.Icon != nil {
		children = append(children,
			layout.Rigid(func(gtx C) D { return e.Icon.Layout(gtx, menuIconSize, col.icon) }),
			layout.Rigid(layout.Spacer{Width: menuGap}.Layout))
	}
	children = append(children, layout.Rigid(m.panel.Text(TextSM, e.Label).In(col.fg).Layout))
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
}

// trailing is the ml-auto group: a shortcut, a submenu's chevron, or a
// checked item's indicator.
// ponytail: the shortcut has no tracking-widest; LabelStyle has no letter
// spacing yet.
func (m menuCore) trailing(gtx C, e MenuEntry, col menuColors) D {
	var w layout.Widget
	switch {
	case e.Kind == MenuItem && e.Shortcut != "":
		w = m.panel.Text(TextXS, e.Shortcut).In(col.shortcut).Layout
	case e.Kind == MenuSubTrigger:
		w = func(gtx C) D { return menuChevron.Layout(gtx, menuIconSize, col.icon) }
	case (e.Kind == MenuCheckboxItem || e.Kind == MenuRadioItem) && e.Checked:
		w = func(gtx C) D { return menuCheck.Layout(gtx, menuIconSize, col.fg) }
	default:
		return D{}
	}
	return layout.W.Layout(gtx, w)
}

func (r menuRow) draw(gtx C, rect image.Rectangle) {
	defer op.Offset(rect.Min).Push(gtx.Ops).Pop()
	size := rect.Size()
	if r.separator {
		line := image.Rect(r.padL, r.padY, size.X-r.padL, r.padY+gtx.Dp(1))
		paint.FillShape(gtx.Ops, r.bg, clip.Rect(line).Op())
		return
	}
	if r.bg.A > 0 {
		paint.FillShape(gtx.Ops, r.bg, clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(RadiusSM)).Op(gtx.Ops))
	}
	left := op.Offset(image.Pt(r.padL, r.padY)).Push(gtx.Ops)
	r.left.Add(gtx.Ops)
	left.Pop()
	right := op.Offset(image.Pt(size.X-r.rightInset-r.rightW, r.padY)).Push(gtx.Ops)
	r.right.Add(gtx.Ops)
	right.Pop()
}
