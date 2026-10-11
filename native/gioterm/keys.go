package gioterm

import (
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
)

// functionKeys type no text, so they are always encoded whatever the modifiers.
var functionKeys = map[key.Name]int{
	key.NameReturn:         keyEnter,
	key.NameEnter:          keyEnter,
	key.NameDeleteBackward: keyBackspace,
	key.NameTab:            keyTab,
	key.NameEscape:         keyEscape,
	key.NameUpArrow:        keyUp,
	key.NameDownArrow:      keyDown,
	key.NameLeftArrow:      keyLeft,
	key.NameRightArrow:     keyRight,
	key.NameHome:           keyHome,
	key.NameEnd:            keyEnd,
	key.NamePageUp:         keyPageUp,
	key.NamePageDown:       keyPageDown,
	key.NameDeleteForward:  keyDelete,
	key.NameF1:             keyF1,
	key.NameF2:             keyF1 + 1,
	key.NameF3:             keyF1 + 2,
	key.NameF4:             keyF1 + 3,
	key.NameF5:             keyF1 + 4,
	key.NameF6:             keyF1 + 5,
	key.NameF7:             keyF1 + 6,
	key.NameF8:             keyF1 + 7,
	key.NameF9:             keyF1 + 8,
	key.NameF10:            keyF1 + 9,
	key.NameF11:            keyF1 + 10,
	key.NameF12:            keyF1 + 11,
}

var punctuationKeys = map[byte]int{
	'`': keyBackquote, '\\': keyBackslash, '[': keyBracketL, ']': keyBracketR,
	',': keyComma, '=': keyEqual, '-': keyMinus, '.': keyPeriod,
	'\'': keyQuote, ';': keySemicolon, '/': keySlash,
}

// characterKey resolves a typing key's name to libghostty's physical key and
// the codepoint it types unshifted. Gio names letters in upper case and other
// symbols by what the layout produced, so a shifted symbol (?, {) with no US
// home is passed as unidentified and lets the encoder fall back to its text.
func characterKey(name key.Name) (k int, unshifted rune, ok bool) {
	if name == key.NameSpace {
		return keySpace, ' ', true
	}
	if len(name) != 1 {
		return 0, 0, false
	}
	c := name[0]
	switch {
	case 'A' <= c && c <= 'Z':
		return keyA + int(c-'A'), rune(c - 'A' + 'a'), true
	case '0' <= c && c <= '9':
		return keyDigit0 + int(c-'0'), rune(c), true
	}
	if k, ok := punctuationKeys[c]; ok {
		return k, rune(c), true
	}
	return keyUnknown, rune(c), true
}

// HandleKey turns a key event into the bytes the program in the terminal
// expects, or nil when the key's text arrives as a key.EditEvent instead.
func (t *Terminal) HandleKey(e key.Event) []byte {
	mods := 0
	if e.Modifiers.Contain(key.ModShift) {
		mods |= modShift
	}
	if e.Modifiers.Contain(key.ModCtrl) {
		mods |= modCtrl
	}
	if e.Modifiers.Contain(key.ModAlt) {
		mods |= modAlt
	}
	if k, ok := functionKeys[e.Name]; ok {
		return t.encodeKey(k, mods, 0, "")
	}
	// Without Ctrl or Alt the key's text arrives as an EditEvent, which also
	// carries what dead keys and compose produced.
	if mods&(modCtrl|modAlt) == 0 {
		return nil
	}
	k, unshifted, ok := characterKey(e.Name)
	if !ok {
		return nil
	}
	text := string(unshifted)
	if mods&modShift != 0 {
		text = strings.ToUpper(text)
	}
	return t.encodeKey(k, mods, unshifted, text)
}

// KeyFilters are the key filters a terminal focused under tag needs: focus
// (for key.EditEvent text), every function key by name, and everything typed
// with Ctrl or Alt, which carries no EditEvent.
func KeyFilters(tag event.Tag) []event.Filter {
	anyMod := key.ModCtrl | key.ModShift | key.ModAlt
	filters := []event.Filter{
		key.FocusFilter{Target: tag},
		key.Filter{Focus: tag, Required: key.ModCtrl, Optional: anyMod},
		key.Filter{Focus: tag, Required: key.ModAlt, Optional: anyMod},
	}
	for name := range functionKeys {
		filters = append(filters, key.Filter{Focus: tag, Name: name, Optional: anyMod})
	}
	return filters
}
