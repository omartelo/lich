// Package snippet cuts the stretch of text around a search hit that a result
// row shows. Both searches share it: the palette's Messages tab, reading a live
// session's transcript (internal/terminal), and the History tab, reading the
// conversation a parked session was indexed with (internal/store).
package snippet

import (
	"strings"
	"unicode"
)

// Width is how many runes of the matched text a row shows. Wide enough to
// recognise the sentence, short enough for one line at palette width.
const Width = 160

// Around renders the stretch of text around the first mention of q as one
// palette line: whitespace collapsed (a message is prose with newlines and code
// in it), a window centred a little ahead of the match so the words leading into
// it stay visible, and an ellipsis on each side that was cut. false when the
// text does not mention q at all.
//
// q is expected lowercased; the text is matched against it without regard to
// case.
func Around(text, q string) (string, bool) {
	flat := strings.Join(strings.Fields(text), " ")
	at := matchRuneIndex(flat, q)
	if at < 0 {
		return "", false
	}
	runes := []rune(flat)
	if len(runes) <= Width {
		return flat, true
	}
	start := max(at-Width/3, 0)
	end := start + Width
	if end > len(runes) {
		end = len(runes)
		start = end - Width
	}
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "…" + snippet
	}
	if end < len(runes) {
		snippet += "…"
	}
	return snippet, true
}

// matchRuneIndex is where text first mentions q (already lowercased), counted in
// runes of text itself; -1 when it does not.
//
// Lowercasing is not length-preserving: "Ⱥ" lowers to a rune one byte longer,
// "İ" to two runes, so an offset read off a lowered copy can point past the end
// of the original. The lowered text is built here alongside the rune of text
// each of its bytes came from, which is what keeps the offset addressing text.
func matchRuneIndex(text, q string) int {
	var lowered strings.Builder
	lowered.Grow(len(text))
	origin := make([]int, 0, len(text))
	for index, r := range []rune(text) {
		before := lowered.Len()
		lowered.WriteRune(unicode.ToLower(r))
		for range lowered.Len() - before {
			origin = append(origin, index)
		}
	}
	at := strings.Index(lowered.String(), q)
	if at < 0 {
		return -1
	}
	return origin[at]
}
