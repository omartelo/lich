package project

import (
	"errors"
	"testing"
)

const sampleDiff = `diff --git a/f.txt b/f.txt
index 1111111..2222222 100644
--- a/f.txt
+++ b/f.txt
@@ -1,4 +1,5 @@
 one
-two
+TWO
+-- looks like a header
 three
 four
\ No newline at end of file
`

func TestParseHunks(t *testing.T) {
	hunks, err := parseHunks(sampleDiff)
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) != 1 || len(hunks[0].lines) != 6 {
		t.Fatalf("hunks = %+v", hunks)
	}
	lines := hunks[0].lines
	if lines[1].kind != '-' || lines[1].old != 2 || lines[2].new != 2 || lines[3].text != "-- looks like a header" {
		t.Errorf("numbering wrong: %+v", lines)
	}
	if !lines[5].noEOL || lines[5].new != 5 {
		t.Errorf("no-EOL marker not attached to the last line: %+v", lines[5])
	}
}

func TestParseHunksMalformed(t *testing.T) {
	for name, diff := range map[string]string{
		"short hunk": "@@ -1,3 +1,3 @@\n one\n",
		"bad header": "@@ -x +1 @@\n one\n",
		"bad line":   "@@ -1 +1 @@\n?one\n",
	} {
		if _, err := parseHunks(diff); err == nil {
			t.Errorf("%s: parsed", name)
		}
	}
}

func TestPartialPatch(t *testing.T) {
	hunks, err := parseHunks(sampleDiff)
	if err != nil {
		t.Fatal(err)
	}
	got, err := partialPatch("f.txt", hunks, []RevertLine{{Side: "new", Line: 3, Text: "-- looks like a header"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "--- a/f.txt\n+++ b/f.txt\n@@ -1,4 +1,5 @@\n one\n TWO\n+-- looks like a header\n three\n four\n\\ No newline at end of file\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestOldLineOf(t *testing.T) {
	hunks, err := parseHunks("@@ -2,2 +2,3 @@\n b\n+x\n c\n@@ -10,2 +11,1 @@\n j\n-k\n")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		line, want int
		ok         bool
	}{{1, 1, true}, {2, 2, true}, {3, 0, false}, {4, 3, true}, {8, 7, true}, {11, 10, true}, {20, 20, true}}
	for _, c := range cases {
		if got, ok := oldLineOf(hunks, c.line); got != c.want || ok != c.ok {
			t.Errorf("oldLineOf(%d) = %d,%v, want %d,%v", c.line, got, ok, c.want, c.ok)
		}
	}
}

func TestPartialPatchEmptySide(t *testing.T) {
	hunks, err := parseHunks("@@ -0,0 +1,2 @@\n+a\n+b\n")
	if err != nil {
		t.Fatal(err)
	}
	picks := []RevertLine{{Side: "new", Line: 1, Text: "a"}, {Side: "new", Line: 2, Text: "b"}}
	got, err := partialPatch("n.txt", hunks, picks)
	if err != nil {
		t.Fatal(err)
	}
	if want := "--- a/n.txt\n+++ b/n.txt\n@@ -0,0 +1,2 @@\n+a\n+b\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPartialPatchStale(t *testing.T) {
	hunks, _ := parseHunks(sampleDiff)
	if _, err := partialPatch("f.txt", hunks, []RevertLine{{Side: "old", Line: 2, Text: "zwei"}}); !errors.Is(err, ErrLinesMoved) {
		t.Errorf("err = %v, want ErrLinesMoved", err)
	}
}
