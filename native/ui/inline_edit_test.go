package ui

import (
	"image"
	"testing"

	"gioui.org/font"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// inlineEditRig drives an InlineEdit through a router, so focus, typed text
// and keys travel the way they do in the app.
type inlineEditRig struct {
	t      *testing.T
	th     *Theme
	edit   *InlineEdit
	router input.Router
}

func newInlineEditRig(t *testing.T, start string) *inlineEditRig {
	r := &inlineEditRig{t: t, th: testTheme(), edit: new(InlineEdit)}
	r.edit.Start(start)
	r.frame()
	r.frame()
	return r
}

func (r *inlineEditRig) frame() InlineEditResult {
	ops := new(op.Ops)
	gtx := layout.Context{
		Ops:         ops,
		Source:      r.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(300, 100)},
	}
	_, res := r.th.InlineEdit(r.edit, TextSM).Weight(font.Medium).Layout(gtx)
	r.router.Frame(ops)
	return res
}

// type_ replaces the selection, which is what the platform's EditEvent range
// says: a bare EditEvent inserts at offset 0.
func (r *inlineEditRig) type_(s string) {
	from, to := r.edit.editor.Selection()
	r.router.Queue(key.EditEvent{Range: key.Range{Start: min(from, to), End: max(from, to)}, Text: s})
	if res := r.frame(); res.Outcome != InlineEditPending {
		r.t.Fatalf("typing ended the edit: %+v", res)
	}
}

func (r *inlineEditRig) blur() InlineEditResult {
	r.router.Source().Execute(key.FocusCmd{})
	return r.frame()
}

func (r *inlineEditRig) press(name key.Name) InlineEditResult {
	r.router.Queue(key.Event{Name: name, State: key.Press})
	return r.frame()
}

func TestInlineEditStartFocusesAndSelectsAll(t *testing.T) {
	r := newInlineEditRig(t, "fix the bug")
	if !r.edit.Active() {
		t.Fatal("not active after Start")
	}
	if !r.router.Source().Focused(&r.edit.editor) {
		t.Error("editor is not focused")
	}
	if got := r.edit.editor.SelectedText(); got != "fix the bug" {
		t.Errorf("selected %q, want the whole text", got)
	}
}

func TestInlineEditEnterCommitsTrimmedValue(t *testing.T) {
	r := newInlineEditRig(t, "old")
	r.type_("  new name ")
	got := r.press(key.NameReturn)
	if got != (InlineEditResult{InlineEditCommitted, "new name"}) {
		t.Errorf("got %+v, want committed %q", got, "new name")
	}
	if r.edit.Active() {
		t.Error("still active after Enter")
	}
	if again := r.frame(); again.Outcome != InlineEditPending {
		t.Errorf("a finished edit reported again: %+v", again)
	}
}

func TestInlineEditEnterOnUnchangedOrEmptyCancels(t *testing.T) {
	same := newInlineEditRig(t, "old")
	if got := same.press(key.NameReturn); got.Outcome != InlineEditCancelled {
		t.Errorf("unchanged: got %+v, want cancelled", got)
	}
	blank := newInlineEditRig(t, "old")
	blank.type_("   ")
	if got := blank.press(key.NameReturn); got.Outcome != InlineEditCancelled {
		t.Errorf("blank: got %+v, want cancelled", got)
	}
}

func TestInlineEditEscapeCancels(t *testing.T) {
	r := newInlineEditRig(t, "old")
	r.type_("new")
	if got := r.press(key.NameEscape); got != (InlineEditResult{Outcome: InlineEditCancelled}) {
		t.Errorf("got %+v, want cancelled with no value", got)
	}
	if r.edit.Active() {
		t.Error("still active after Escape")
	}
}

func TestInlineEditBlurCommits(t *testing.T) {
	r := newInlineEditRig(t, "old")
	r.type_("new")
	got := r.blur()
	if got != (InlineEditResult{InlineEditCommitted, "new"}) {
		t.Errorf("got %+v, want committed %q", got, "new")
	}
}

func TestInlineEditBlurOnUnchangedCancels(t *testing.T) {
	r := newInlineEditRig(t, "old")
	if got := r.blur(); got.Outcome != InlineEditCancelled {
		t.Errorf("got %+v, want cancelled", got)
	}
}

// The field replaces a label in a row: a different height or baseline would
// make the row jump when editing starts.
func TestInlineEditIsLaidOutLikeTheLabel(t *testing.T) {
	th := testTheme()
	edit := new(InlineEdit)
	edit.Start("fix the bug")
	field := func(gtx C) D {
		d, _ := th.InlineEdit(edit, TextSM).Weight(font.Medium).Layout(gtx)
		return d
	}
	label := th.Text(TextSM, "fix the bug").Weight(font.Medium).Layout
	got, want := layoutAt(field, 0, 300), layoutAt(label, 0, 300)
	if got.Size.Y != want.Size.Y || got.Baseline != want.Baseline {
		t.Errorf("field is %d tall with baseline %d, label %d and %d", got.Size.Y, got.Baseline, want.Size.Y, want.Baseline)
	}
	if got.Size.X != 300 {
		t.Errorf("width %d, want w-full (300)", got.Size.X)
	}
}
