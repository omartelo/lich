package quota

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"testing"
	"time"
)

// agyUsageReply is agy 1.2.5's answer to `/usage`, measured on 2026-10-08, with
// the Gemini pool given a 5h bucket and some spend so both windows and the
// group-naming rule are exercised.
const agyUsageReply = `{"conversation_id":"","status":"SUCCESS","response":"…","duration_seconds":0,"num_turns":0,` +
	`"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0},"command":{"name":"usage","data":{` +
	`"description":"Within each group, models share a weekly limit.","groups":[` +
	`{"name":"Gemini Models","buckets":[` +
	`{"id":"gemini-weekly","name":"Weekly Limit Remaining","window":"weekly","remaining_fraction":0.75,"reset_time":"2026-10-15T15:21:23Z"},` +
	`{"id":"gemini-5h","name":"5h Limit Remaining","window":"5h","remaining_fraction":0.5,"reset_time":"2026-10-08T20:00:00Z"}]},` +
	`{"name":"Claude and GPT models","buckets":[` +
	`{"id":"3p-weekly","name":"Weekly Limit Remaining","window":"weekly","remaining_fraction":1,"reset_time":"2026-10-15T15:21:23Z"},` +
	`{"id":"3p-5h","name":"5h Limit Remaining","window":"5h","remaining_fraction":1,"disabled":true}]}]}}}`

// fakeAgy answers --version with version and the usage command with reply,
// recording every argv it was run with.
func fakeAgy(version, reply string, replyErr error) (func([]string, ...string) ([]byte, error), *[][]string) {
	var runs [][]string
	return func(_ []string, args ...string) ([]byte, error) {
		runs = append(runs, args)
		if slices.Equal(args, []string{"--version"}) {
			return []byte(version + "\n"), nil
		}
		return []byte(reply), replyErr
	}, &runs
}

func agyService(run func([]string, ...string) ([]byte, error)) *Service {
	s := newService("", "", time.Now())
	s.runAgy = run
	return s
}

func TestAntigravityReadsEveryMeteredBucket(t *testing.T) {
	run, runs := fakeAgy("1.2.5", "agy: starting language server\n"+agyUsageReply+"\n", nil)
	got := agyService(run).antigravityPlan(lichEnv())

	if got.Status != StatusOK || got.Provider != "antigravity" || got.Name != "Antigravity" {
		t.Fatalf("plan = %+v, want an ok Antigravity reading", got)
	}
	want := []Window{
		{Label: "Gemini Models · Weekly Limit Remaining", Seconds: weeklyWindow, Percent: 25, ResetsAt: "2026-10-15T15:21:23Z"},
		{Label: "Gemini Models · 5h Limit Remaining", Seconds: sessionWindow, Percent: 50, ResetsAt: "2026-10-08T20:00:00Z"},
		{Label: "Claude and GPT models", Seconds: weeklyWindow, Percent: 0, ResetsAt: "2026-10-15T15:21:23Z"},
	}
	if !slices.Equal(got.Windows, want) {
		t.Errorf("windows = %+v, want %+v", got.Windows, want)
	}
	if usage := (*runs)[1]; !slices.Contains(usage, "/usage") || slices.Contains(usage, "--disable-slash-commands") {
		t.Errorf("usage argv = %q, want /usage run as a slash command", usage)
	}
}

func TestAntigravityWithoutTheCLIIsLeftOut(t *testing.T) {
	missing := func([]string, ...string) ([]byte, error) { return nil, exec.ErrNotFound }
	if got := agyService(missing).antigravityPlan(lichEnv()); got.Provider != "" {
		t.Errorf("plan = %+v, want none for a machine with no agy", got)
	}
	if got := newService("", "", time.Now()).antigravityPlan(lichEnv()); got.Provider != "" {
		t.Errorf("plan = %+v, want none with no runner wired", got)
	}
}

func TestAntigravityNeverAsksAnAgyThatSpendsATurn(t *testing.T) {
	for _, version := range []string{"1.1.10", "garbage", ""} {
		run, runs := fakeAgy(version, agyUsageReply, nil)
		got := agyService(run).antigravityPlan(lichEnv())
		if got.Status != StatusError {
			t.Errorf("version %q: status = %q, want error", version, got.Status)
		}
		if len(*runs) != 1 {
			t.Errorf("version %q: runs = %q, want only the version probe", version, *runs)
		}
	}
}

func TestAntigravityStopsAskingOnceUsageRanAModelTurn(t *testing.T) {
	turn := `{"conversation_id":"c-1","status":"SUCCESS","response":"Here is your usage…","num_turns":1}`
	run, runs := fakeAgy("1.2.5", turn, nil)
	s := agyService(run)

	if got := s.antigravityPlan(lichEnv()); got.Status != StatusError {
		t.Fatalf("status = %q, want error", got.Status)
	}
	s.antigravityPlan(lichEnv())
	if len(*runs) != 2 {
		t.Errorf("runs = %q, want no run at all after the turn was spent", *runs)
	}
}

func TestAntigravitySignedOut(t *testing.T) {
	for _, out := range []string{
		`AGY_ERROR: {"status":"UNAUTHENTICATED","error_code":401}`,
		"Error: Not logged in. Run agy login.",
	} {
		run, _ := fakeAgy("1.2.5", out, errors.New("exit status 1"))
		if got := agyService(run).antigravityPlan(lichEnv()); got.Status != StatusSignedOut {
			t.Errorf("%q: status = %q, want signed out", out, got.Status)
		}
	}
}

func TestAntigravityFailures(t *testing.T) {
	for name, tc := range map[string]struct {
		out string
		err error
	}{
		"timeout":     {agyUsageReply, context.DeadlineExceeded},
		"no reply":    {"something else entirely", errors.New("exit status 2")},
		"no buckets":  {`{"status":"SUCCESS","num_turns":0,"command":{"name":"usage","data":{"groups":[]}}}`, nil},
		"other reply": {`{"status":"SUCCESS","num_turns":0,"command":{"name":"help","data":{}}}`, nil},
	} {
		run, _ := fakeAgy("1.2.5", tc.out, tc.err)
		if got := agyService(run).antigravityPlan(lichEnv()); got.Status != StatusError {
			t.Errorf("%s: status = %q, want error", name, got.Status)
		}
	}
}

func TestAntigravityHiddenSessionIsUnknownWithoutRunningAgy(t *testing.T) {
	run, runs := fakeAgy("1.2.5", agyUsageReply, nil)
	if got := agyService(run).antigravityPlan(Account{}); got.Status != StatusUnknown {
		t.Errorf("status = %q, want unknown", got.Status)
	}
	if len(*runs) != 0 {
		t.Errorf("runs = %q, want none for an account lich cannot read", *runs)
	}
}

func TestAntigravityRunsInTheSessionsEnvironment(t *testing.T) {
	var envs [][]string
	run := func(env []string, args ...string) ([]byte, error) {
		envs = append(envs, env)
		if args[0] == "--version" {
			return []byte("1.2.5"), nil
		}
		return []byte(agyUsageReply), nil
	}
	agyService(run).antigravityPlan(Account{Read: true, Env: map[string]string{"HOME": "/home/session"}})
	for _, env := range envs {
		if !slices.Equal(env, []string{"HOME=/home/session"}) {
			t.Errorf("env = %q, want the session's own", env)
		}
	}
	if (Account{Read: true}).environ() != nil {
		t.Error("the machine-wide reading must inherit lich's environment")
	}
}

func TestVersionAtLeast(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"1.1.11", true},
		{"1.2.5", true},
		{"2.0.0", true},
		{"1.1.10", false},
		{"0.9.99", false},
		{"v1.2.5", false},
		{"", false},
	} {
		if got := versionAtLeast(tc.version, agyMinUsageVersion); got != tc.want {
			t.Errorf("versionAtLeast(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
	if versionAtLeast("1.2.5", "nonsense") {
		t.Error("an unreadable floor must never pass")
	}
}

func TestRunAgyReportsAMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := runAgy(nil, "--version"); !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("err = %v, want exec.ErrNotFound", err)
	}
}
