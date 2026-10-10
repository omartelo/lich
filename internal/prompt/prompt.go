// Package prompt holds the natural-language text lich types into an agent
// session, in every language a user can pick for it. The UI has its own
// language and its own catalogs (frontend/src/lib/i18n); the two settings are
// independent because people often run the interface in one language and talk
// to their agents in another.
//
// A catalog is a struct rather than a map of string keys so that a wrong name
// is a compile error at the call site, and a locale that misses a field is
// caught by TestCatalogParity. Each field is a fmt format: use explicit
// argument indexes (%[2]s) when a translation needs the arguments in another
// order.
package prompt

// Lang is a prompt language, spelled as a BCP 47 tag: the same value is stored
// in the settings table, exported to every session as EnvVar, and offered by
// the frontend's select.
type Lang string

const (
	English      Lang = "en"
	PortugueseBR Lang = "pt-BR"
	Spanish      Lang = "es"
)

// Langs is every language a catalog exists for, English first as the default.
var Langs = []Lang{English, PortugueseBR, Spanish}

// EnvVar is the variable lich exports into every session PTY with the prompt
// language at spawn time. Text composed outside the lich process (the lich CLI,
// its MCP server, the companion plugin's hooks) reads the language from here,
// because it has no way to read the setting itself.
const EnvVar = "LICH_PROMPT_LANG"

// Parse reads a stored or exported language tag. An empty or unknown tag is
// English: absent is the default by contract, and a tag from a newer build
// must not stop an older one from talking to its agents.
func Parse(tag string) Lang {
	for _, lang := range Langs {
		if string(lang) == tag {
			return lang
		}
	}
	return English
}

var catalogs = map[Lang]*Catalog{
	English:      &en,
	PortugueseBR: &ptBR,
	Spanish:      &es,
}

// For is the catalog of a language. A Lang that came through Parse always has
// one; a hand-built Lang with no catalog panics, which is a programming error.
func For(lang Lang) *Catalog {
	catalog, ok := catalogs[lang]
	if !ok {
		panic("prompt: no catalog for language " + string(lang))
	}
	return catalog
}

// Plural is a text that changes with a count. Only the two forms both catalogs
// need exist; a language with more CLDR forms adds fields when it arrives.
type Plural struct {
	One   string
	Other string
}

// Pick is the form of p this catalog's language uses for n.
func (c *Catalog) Pick(n int, p Plural) string {
	if c.isOne(n) {
		return p.One
	}
	return p.Other
}

// isOneEnglish, isOnePortuguese and isOneSpanish are the CLDR "one" rules for
// integers: Portuguese counts zero as singular, English and Spanish do not.
func isOneEnglish(n int) bool    { return n == 1 }
func isOnePortuguese(n int) bool { return n == 0 || n == 1 }
func isOneSpanish(n int) bool    { return n == 1 }
