package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/doctor"
)

// doctorClient runs `lich doctor` against a canned walk: the real one binds the
// pinned port and opens the workspace database, neither of which a unit test may
// do to the machine it runs on.
func doctorClient(t *testing.T, checks []doctor.Check, err error) (*client, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	return &client{
		env:      func(string) string { return "" },
		version:  "v1.2.3",
		stdout:   &stdout,
		stderr:   &stderr,
		diagnose: func() ([]doctor.Check, error) { return checks, err },
	}, &stdout, &stderr
}

func TestDoctorPrintsTheReportAndExitsZeroWhenNothingStopsALaunch(t *testing.T) {
	c, stdout, stderr := doctorClient(t, []doctor.Check{
		{Name: "home", Status: doctor.OK, Detail: "/home/u/.config/lich is writable"},
		{Name: "store", Status: doctor.Skip, Detail: "held by the running lich (pid 42)"},
	}, nil)

	if code := dispatch([]string{"doctor"}, c); code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr.String())
	}
	for _, want := range []string{"lich v1.2.3", "home", "store", "nothing is in the way"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("report is missing %q: %q", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Errorf("a healthy report wrote to stderr: %q", stderr.String())
	}
}

func TestDoctorExitsNonZeroOnACheckThatStopsALaunch(t *testing.T) {
	c, stdout, stderr := doctorClient(t, []doctor.Check{
		{Name: "browser", Status: doctor.Fail, Detail: "no chromium-family browser found"},
	}, nil)

	if code := dispatch([]string{"doctor"}, c); code != 1 {
		t.Fatalf("exit = %d, want 1 — a script reads this", code)
	}
	if !strings.Contains(stdout.String(), "no chromium-family browser found") {
		t.Errorf("stdout = %q", stdout.String())
	}
	// The report already said it. Saying it again on stderr, worse, is the
	// reason this command does not go through run().
	if stderr.Len() != 0 {
		t.Errorf("the failure was repeated on stderr: %q", stderr.String())
	}
}

func TestDoctorReportsAWalkItCouldNotStart(t *testing.T) {
	c, stdout, stderr := doctorClient(t, nil, errors.New("resolve config directory: no home"))

	if code := dispatch([]string{"doctor"}, c); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no home") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("a walk that never ran still printed a report: %q", stdout.String())
	}
}

func TestDoctorTakesNoArguments(t *testing.T) {
	c, stdout, stderr := doctorClient(t, []doctor.Check{{Name: "home", Status: doctor.OK}}, nil)

	if code := dispatch([]string{"doctor", "--everything"}, c); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "lich: ") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("a rejected command still printed a report: %q", stdout.String())
	}

	c, stdout, stderr = doctorClient(t, []doctor.Check{{Name: "home", Status: doctor.OK}}, nil)
	if code := dispatch([]string{"doctor", "everything"}, c); code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "usage: lich doctor") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("a rejected command still printed a report: %q", stdout.String())
	}
}

func TestDoctorIsInTheHelp(t *testing.T) {
	f := newFakeLich(t, `null`)
	_, stdout, _ := run(t, f, "help")
	if !strings.Contains(stdout, "lich doctor") {
		t.Errorf("help does not document doctor: %q", stdout)
	}
}
