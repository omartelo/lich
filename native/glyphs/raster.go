// Package glyphs rasterizes a monospace font family into per-character
// coverage masks, for Gio views that compose text into images on the CPU
// instead of drawing Gio text paths.
package glyphs

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"os/exec"
	"slices"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// fontStyles are the fontconfig styles of a family, indexed by styleIndex.
var fontStyles = [4]string{"regular", "bold", "italic", "bold:italic"}

func styleIndex(k Key) int {
	i := 0
	if k.Bold {
		i |= 1
	}
	if k.Italic {
		i |= 2
	}
	return i
}

// Key names one glyph: a character in one of the family's four styles.
type Key struct {
	R            rune
	Bold, Italic bool
}

type fontFace struct {
	sf   *sfnt.Font
	face font.Face
}

// Mask is a glyph's coverage, placed relative to its cell's top-left.
// Img is nil for a glyph that draws nothing.
type Mask struct {
	Img *image.Alpha
	Off image.Point
}

// Rasterizer turns characters into coverage masks on the CPU, once each, for
// the renderers that keep glyphs as images instead of Gio text paths. It also
// owns the fallback Gio lacks: a character the family does not cover is drawn
// from whichever installed font fontconfig says covers it.
type Rasterizer struct {
	family   string
	px       float64
	styles   [4]*fontFace
	byFile   map[string]*fontFace
	byRune   map[rune]*fontFace
	masks    map[Key]*Mask
	buf      sfnt.Buffer
	cellW    int
	cellH    int
	ascent   int
	fcLookup func(r rune) (string, error)
}

// New loads the four styles fontconfig resolves for family. Call SetSize
// before Mask or Cell.
func New(family string) (*Rasterizer, error) {
	r := &Rasterizer{family: family, byFile: map[string]*fontFace{}, fcLookup: fontconfigCovering}
	for i, style := range fontStyles {
		path, err := fcMatch(family + ":" + style)
		if err != nil {
			return nil, err
		}
		if r.styles[i], err = r.load(path); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func fcMatch(pattern string) (string, error) {
	out, err := exec.Command("fc-match", "-f", "%{file}", pattern).Output()
	if err != nil {
		return "", fmt.Errorf("fc-match %s: %w", pattern, err)
	}
	return string(out), nil
}

// fontconfigCovering names a font file that has a glyph for r.
func fontconfigCovering(r rune) (string, error) {
	charset := fmt.Sprintf(":charset=%x", r)
	out, err := exec.Command("fc-list", charset, "file").Output()
	if err != nil {
		return "", fmt.Errorf("fc-list %s: %w", charset, err)
	}
	covering := parseFcListFiles(string(out))
	if len(covering) == 0 {
		return "", nil
	}
	// fc-list knows who covers the character but in no useful order; fc-match
	// ranks every font against a plain monospace, so the first covering font
	// in its ranking is the closest regular face.
	ranked, err := exec.Command("fc-match", "-s", "-f", "%{file}\n", "monospace").Output()
	if err != nil {
		return "", fmt.Errorf("fc-match -s monospace: %w", err)
	}
	for f := range strings.Lines(string(ranked)) {
		if f = strings.TrimSpace(f); slices.Contains(covering, f) {
			return f, nil
		}
	}
	return covering[0], nil
}

// parseFcListFiles reads `fc-list <pattern> file`, one "path: " per line.
func parseFcListFiles(out string) []string {
	var files []string
	for line := range strings.Lines(out) {
		if f := strings.TrimSuffix(strings.TrimSpace(line), ":"); f != "" {
			files = append(files, f)
		}
	}
	return files
}

func (r *Rasterizer) load(path string) (*fontFace, error) {
	if f, ok := r.byFile[path]; ok {
		return f, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	coll, err := opentype.ParseCollection(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	sf, err := coll.Font(0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	f := &fontFace{sf: sf}
	if r.px > 0 {
		if f.face, err = opentype.NewFace(sf, &opentype.FaceOptions{Size: r.px, DPI: 72}); err != nil {
			return nil, err
		}
	}
	r.byFile[path] = f
	return f, nil
}

// SetSize re-creates every face at px pixels per em and reports whether the
// size changed, which drops every cached mask.
func (r *Rasterizer) SetSize(px float64) (bool, error) {
	if px == r.px {
		return false, nil
	}
	r.px = px
	for _, f := range r.byFile {
		face, err := opentype.NewFace(f.sf, &opentype.FaceOptions{Size: px, DPI: 72})
		if err != nil {
			return false, err
		}
		f.face = face
	}
	m := r.styles[0].face.Metrics()
	adv, _ := r.styles[0].face.GlyphAdvance('M')
	r.cellW = adv.Ceil()
	r.ascent = m.Ascent.Ceil()
	r.cellH = (m.Ascent + m.Descent).Ceil()
	r.masks = map[Key]*Mask{}
	r.byRune = map[rune]*fontFace{}
	return true, nil
}

func (r *Rasterizer) covers(f *fontFace, c rune) bool {
	gi, err := f.sf.GlyphIndex(&r.buf, c)
	return err == nil && gi != 0
}

// faceFor picks the family's face for k, or a fallback that covers its
// character. It returns nil when nothing installed does.
func (r *Rasterizer) faceFor(k Key) (*fontFace, error) {
	primary := r.styles[styleIndex(k)]
	if r.covers(primary, k.R) {
		return primary, nil
	}
	if f, ok := r.byRune[k.R]; ok {
		return f, nil
	}
	path, err := r.fcLookup(k.R)
	if err != nil {
		return nil, err
	}
	var f *fontFace
	if path != "" {
		if f, err = r.load(path); err != nil {
			return nil, err
		}
		if !r.covers(f, k.R) {
			f = nil
		}
	}
	r.byRune[k.R] = f
	return f, nil
}

// Mask returns k's coverage, rasterizing it on first use.
func (r *Rasterizer) Mask(k Key) (*Mask, error) {
	if m, ok := r.masks[k]; ok {
		return m, nil
	}
	m := &Mask{}
	f, err := r.faceFor(k)
	if err != nil {
		return nil, err
	}
	if f != nil {
		dr, src, sp, _, ok := f.face.Glyph(fixed.P(0, r.ascent), k.R)
		if ok && !dr.Empty() {
			a := image.NewAlpha(image.Rect(0, 0, dr.Dx(), dr.Dy()))
			draw.Draw(a, a.Bounds(), src, sp, draw.Src)
			m.Img, m.Off = a, dr.Min
		}
	}
	r.masks[k] = m
	return m, nil
}

// CellW is the width of one monospace cell in pixels at the current size.
func (r *Rasterizer) CellW() int { return r.cellW }

// CellH is the height of one line in pixels at the current size.
func (r *Rasterizer) CellH() int { return r.cellH }
