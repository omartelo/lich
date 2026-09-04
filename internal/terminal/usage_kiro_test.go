package terminal

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// kiroSessionFile writes a Kiro session metadata file holding state and returns
// its path. The shape is the one a real 2.21.0 session wrote; the turn-by-turn
// metadata a real file also carries is left out, because none of it is read.
func kiroSessionFile(t *testing.T, state string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	body := `{"session_id":"s1","cwd":"/w","session_state":` + state + `}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	return path
}

// TestKiroContextUsageReadsThePercentage proves the readout is taken from the
// percentage Kiro records rather than from a token count it does not, and that
// the percent lich shows is the one Kiro's own TUI drew: a session carrying
// 0.74200004 against a 200k window was displayed as "1%".
func TestKiroContextUsageReadsThePercentage(t *testing.T) {
	path := kiroSessionFile(t, `{"rts_model_state":{
		"model_info":{"model_id":"auto","context_window_tokens":200000},
		"context_usage_percentage":0.74200004}}`)

	got, ok := kiroContextUsage(path)
	if !ok {
		t.Fatalf("kiroContextUsage = _, false, want ok")
	}
	if got.percent != 1 {
		t.Errorf("percent = %d, want 1 (what Kiro's own TUI drew)", got.percent)
	}
	if got.window != 200000 {
		t.Errorf("window = %d, want 200000", got.window)
	}
	if got.model != "auto" {
		t.Errorf("model = %q, want %q", got.model, "auto")
	}
	// Derived from the percentage against the window, because Kiro files every
	// token count as zero even on a turn that spent tokens. The tooltip reads
	// "tokens / window", so a literal zero there would contradict the ring.
	if got.tokens != 1484 {
		t.Errorf("tokens = %d, want 1484 (0.742%% of 200000)", got.tokens)
	}
}

// TestKiroContextUsageRounding pins the boundary rather than deriving it from
// the same expression the code uses: a half-percent rounds up, and a window
// share too small to reach one percent still reports the tokens it spent.
func TestKiroContextUsageRounding(t *testing.T) {
	tests := []struct {
		name    string
		percent string
		window  int
		want    int
	}{
		{"just under a half rounds down", "49.4", 1000, 49},
		{"a half rounds up", "49.5", 1000, 50},
		{"just over a half rounds up", "49.6", 1000, 50},
		{"a full window", "100", 1000, 100},
		{"an untouched window", "0", 1000, 0},
	}
	for _, tt := range tests {
		path := kiroSessionFile(t, `{"rts_model_state":{
			"model_info":{"model_id":"auto","context_window_tokens":`+strconv.Itoa(tt.window)+`},
			"context_usage_percentage":`+tt.percent+`}}`)
		got, ok := kiroContextUsage(path)
		if !ok {
			t.Fatalf("%s: kiroContextUsage = _, false, want ok", tt.name)
		}
		if got.percent != tt.want {
			t.Errorf("%s: percent = %d, want %d", tt.name, got.percent, tt.want)
		}
	}
}

// TestKiroContextUsageMisses proves every absence reads as "keep the last
// value" rather than as a window of zero. A session that has been opened but
// not yet asked anything writes both fields as null, which is the common one:
// repainting the ring at 0% there would blank a figure the last turn earned.
func TestKiroContextUsageMisses(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{"opened but never asked", `{"rts_model_state":{"model_info":null,"context_usage_percentage":null}}`},
		{"a model with no window yet", `{"rts_model_state":{"model_info":{"model_id":"auto","context_window_tokens":0},"context_usage_percentage":1}}`},
		{"a window but no percentage", `{"rts_model_state":{"model_info":{"model_id":"auto","context_window_tokens":200000},"context_usage_percentage":null}}`},
		{"a negative percentage", `{"rts_model_state":{"model_info":{"model_id":"auto","context_window_tokens":200000},"context_usage_percentage":-1}}`},
		{"no model state at all", `{}`},
	}
	for _, tt := range tests {
		path := kiroSessionFile(t, tt.state)
		if _, ok := kiroContextUsage(path); ok {
			t.Errorf("%s: kiroContextUsage = _, true, want a miss", tt.name)
		}
	}
}

// TestKiroContextUsageUnreadable proves a file that is missing or not JSON is a
// miss rather than a panic — a session file is written by another process and
// can be read mid-write.
func TestKiroContextUsageUnreadable(t *testing.T) {
	if _, ok := kiroContextUsage(filepath.Join(t.TempDir(), "absent.json")); ok {
		t.Errorf("kiroContextUsage(absent) = _, true, want a miss")
	}
	path := filepath.Join(t.TempDir(), "half.json")
	if err := os.WriteFile(path, []byte(`{"session_state":{"rts_mo`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, ok := kiroContextUsage(path); ok {
		t.Errorf("kiroContextUsage(truncated) = _, true, want a miss")
	}
}
