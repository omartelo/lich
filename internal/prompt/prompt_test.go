package prompt

import (
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"testing"
)

// TestCatalogParity is the guard a struct cannot give: every locale fills every
// field, and fills it with the arguments English expects, so a translation can
// reorder its arguments but never drop or invent one.
func TestCatalogParity(t *testing.T) {
	source := reflect.ValueOf(en)
	for _, lang := range Langs {
		locale := reflect.ValueOf(*For(lang))
		for i := range source.NumField() {
			field := source.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			for suffix, pair := range texts(source.Field(i), locale.Field(i)) {
				name := field.Name + suffix
				if pair[1] == "" {
					t.Errorf("%s: %s is missing", lang, name)
					continue
				}
				if want, got := verbs(pair[0]), verbs(pair[1]); !slices.Equal(want, got) {
					t.Errorf("%s: %s takes %v, English takes %v", lang, name, got, want)
				}
			}
		}
	}
}

// texts pairs a field's English text with the locale's, one pair per plural
// form for a Plural.
func texts(source, locale reflect.Value) map[string][2]string {
	if plural, ok := source.Interface().(Plural); ok {
		other := locale.Interface().(Plural)
		return map[string][2]string{
			".One":   {plural.One, other.One},
			".Other": {plural.Other, other.Other},
		}
	}
	return map[string][2]string{"": {source.String(), locale.String()}}
}

var verbPattern = regexp.MustCompile(`%(\[(\d+)\])?[-+# 0]*[0-9]*(\.[0-9]+)?([a-zA-Z%])`)

// verbs lists the arguments a format consumes as "index:verb", sorted, with
// implicit indexes resolved the way fmt resolves them.
func verbs(format string) []string {
	var found []string
	next := 1
	for _, m := range verbPattern.FindAllStringSubmatch(format, -1) {
		if m[4] == "%" {
			continue
		}
		if m[2] != "" {
			next, _ = strconv.Atoi(m[2])
		}
		found = append(found, strconv.Itoa(next)+":"+m[4])
		next++
	}
	slices.Sort(found)
	return slices.Compact(found)
}

func TestParse(t *testing.T) {
	for tag, want := range map[string]Lang{
		"en": English, "pt-BR": PortugueseBR, "": English, "fr": English, "pt-br": English,
	} {
		if got := Parse(tag); got != want {
			t.Errorf("Parse(%q) = %q, want %q", tag, got, want)
		}
	}
}

func TestForPanicsOnALangWithNoCatalog(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("For(\"xx\") did not panic")
		}
	}()
	For("xx")
}

func TestPickFollowsEachLanguagesPluralRule(t *testing.T) {
	forms := Plural{One: "one", Other: "other"}
	cases := []struct {
		lang Lang
		n    int
		want string
	}{
		{English, 0, "other"}, {English, 1, "one"}, {English, 2, "other"},
		{PortugueseBR, 0, "one"}, {PortugueseBR, 1, "one"}, {PortugueseBR, 2, "other"},
	}
	for _, tc := range cases {
		if got := For(tc.lang).Pick(tc.n, forms); got != tc.want {
			t.Errorf("%s Pick(%d) = %q, want %q", tc.lang, tc.n, got, tc.want)
		}
	}
}
