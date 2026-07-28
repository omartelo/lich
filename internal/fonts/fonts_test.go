package fonts

import (
	"os/exec"
	"reflect"
	"runtime"
	"testing"
)

func TestListSmoke(t *testing.T) {
	// Windows reads the registry, which is always there; everywhere else the
	// enumeration is fc-list, and a machine without fontconfig has nothing to say.
	if runtime.GOOS != "windows" {
		if _, err := exec.LookPath("fc-list"); err != nil {
			t.Skip("fc-list not installed")
		}
	}
	got, err := New().List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) == 0 {
		t.Error("List() returned no families")
	}
}

func TestParseFamilies(t *testing.T) {
	out := "Fira Code\nMonaco\nFira Code\nNoto Sans,Noto Sans CJK\n\n  \n"
	got := parseFamilies(out)
	want := []string{"Fira Code", "Monaco", "Noto Sans"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseFamilies = %v, want %v", got, want)
	}
}

// TestParseRegistryFamilies proves the Windows Fonts key's value names collapse
// onto the family list the picker offers: the format parenthetical goes, faces
// of one family fold together, a multi-family entry splits, and a family whose
// name merely ends in a face-like word survives whole.
func TestParseRegistryFamilies(t *testing.T) {
	names := []string{
		"Arial (TrueType)",
		"Arial Bold (TrueType)",
		"Arial Bold Italic (TrueType)",
		"Arial Black (TrueType)",
		"Cascadia Mono (TrueType)",
		"Segoe UI Light (TrueType)",
		"MS Gothic & MS UI Gothic & MS PGothic (TrueType)",
		"Courier 10,12,15 (VGA res)",
		"   ",
		"",
	}
	want := []string{
		"Arial",
		"Arial Black",
		"Cascadia Mono",
		"Courier 10,12,15",
		"MS Gothic",
		"MS PGothic",
		"MS UI Gothic",
		"Segoe UI Light",
	}
	if got := parseRegistryFamilies(names); !reflect.DeepEqual(got, want) {
		t.Errorf("parseRegistryFamilies =\n%v\nwant\n%v", got, want)
	}
}

// TestTrimFontFacesKeepsALoneFaceWord proves the trim never empties a name: a
// family called "Bold" is a family, not a face of nothing.
func TestTrimFontFacesKeepsALoneFaceWord(t *testing.T) {
	if got := trimFontFaces("Bold"); got != "Bold" {
		t.Errorf("trimFontFaces(\"Bold\") = %q, want %q", got, "Bold")
	}
}
