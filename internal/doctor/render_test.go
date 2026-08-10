package doctor

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func rendered(t *testing.T, checks []Check) string {
	t.Helper()
	var buf bytes.Buffer
	Render(&buf, "v1.2.3", checks)
	return buf.String()
}

func TestRenderNamesTheBuildEveryCheckAndTheTotal(t *testing.T) {
	page := rendered(t, []Check{
		{Name: "home", Status: OK, Detail: "/home/u/.config/lich is writable", Took: 2 * time.Millisecond},
		{Name: "store", Status: Skip, Detail: "held by the running lich (pid 42)", Took: 300 * time.Microsecond},
	})

	for _, want := range []string{
		"lich v1.2.3 — ",
		"ok    home",
		"/home/u/.config/lich is writable",
		"skip  store",
		"2ms",
		"<1ms", // faster than a millisecond, and never printed as 0s
		"total",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("report is missing %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, " 0s") {
		t.Errorf("a sub-millisecond check printed as 0s:\n%s", page)
	}
}

func TestTheVerdictIsWhatTheReaderActsOn(t *testing.T) {
	cases := []struct {
		name   string
		checks []Check
		want   string
	}{
		{"all clear", []Check{{Status: OK}}, "lich starts here — nothing is in the way."},
		{
			"a skip is not a pass",
			[]Check{{Status: OK}, {Status: Skip}},
			"lich starts here — nothing is in the way.",
		},
		{
			"one warning",
			[]Check{{Status: OK}, {Status: Warn}},
			"lich starts. 1 warning above limits something it offers.",
		},
		{
			"two warnings",
			[]Check{{Status: Warn}, {Status: Warn}},
			"lich starts. 2 warnings above limit something it offers.",
		},
		{"one failure", []Check{{Status: Fail}}, "1 check above stops a launch here. Fix it first."},
		{
			"two failures",
			[]Check{{Status: Fail}, {Status: Fail}},
			"2 checks above stop a launch here. Fix them first.",
		},
		{
			"a failure outranks a warning",
			[]Check{{Status: Warn}, {Status: Fail}},
			"1 check above stops a launch here. Fix it first.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := verdict(tc.checks); got != tc.want {
				t.Errorf("verdict = %q, want %q", got, tc.want)
			}
			if !strings.Contains(rendered(t, tc.checks), tc.want) {
				t.Error("the verdict never reached the rendered report")
			}
		})
	}
}

func TestTookRendersWholeMilliseconds(t *testing.T) {
	cases := map[time.Duration]string{
		0:                       "<1ms",
		999 * time.Microsecond:  "<1ms",
		time.Millisecond:        "1ms",
		1500 * time.Microsecond: "2ms",
		2 * time.Second:         "2s",
	}
	for d, want := range cases {
		if got := took(d); got != want {
			t.Errorf("took(%v) = %q, want %q", d, got, want)
		}
	}
}
