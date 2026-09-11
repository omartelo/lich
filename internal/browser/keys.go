package browser

import (
	"fmt"
	"strings"

	"github.com/chromedp/chromedp/kb"
)

// namedKeys maps the names agents and `lich browser press` use to the
// single-rune sequences chromedp's KeyEvent understands (kb package).
// Aliases are folded in normalizeKey; this table is the canonical spellings.
var namedKeys = map[string]string{
	"enter":      kb.Enter,
	"tab":        kb.Tab,
	"escape":     kb.Escape,
	"backspace":  kb.Backspace,
	"delete":     kb.Delete,
	"space":      " ",
	"arrowup":    kb.ArrowUp,
	"arrowdown":  kb.ArrowDown,
	"arrowleft":  kb.ArrowLeft,
	"arrowright": kb.ArrowRight,
	"home":       kb.Home,
	"end":        kb.End,
	"pageup":     kb.PageUp,
	"pagedown":   kb.PageDown,
}

// keyAliases fold common short names onto the table above.
var keyAliases = map[string]string{
	"return": "enter",
	"esc":    "escape",
	"bs":     "backspace",
	"del":    "delete",
	"up":     "arrowup",
	"down":   "arrowdown",
	"left":   "arrowleft",
	"right":  "arrowright",
	"pgup":   "pageup",
	"pgdn":   "pagedown",
}

// keySequence turns a named key into the string chromedp.KeyEvent expects.
// Unknown names are refused so agents cannot invent chords or paste text here —
// that is what Type is for.
func keySequence(name string) (string, error) {
	n := normalizeKey(name)
	if n == "" {
		return "", fmt.Errorf("press needs a key name (Enter, Tab, Escape, …)")
	}
	seq, ok := namedKeys[n]
	if !ok {
		return "", fmt.Errorf("unknown key %q — use Enter, Tab, Escape, Backspace, Delete, Space, ArrowUp/Down/Left/Right, Home, End, PageUp, PageDown", name)
	}
	return seq, nil
}

func normalizeKey(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "-", "")
	n = strings.ReplaceAll(n, "_", "")
	n = strings.ReplaceAll(n, " ", "")
	if alias, ok := keyAliases[n]; ok {
		return alias
	}
	return n
}
