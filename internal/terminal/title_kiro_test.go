package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

// TestKiroMetadataTitle proves the name is taken from the `title` Kiro files
// beside the rest of a conversation's metadata, and that every way it can be
// absent reads as silence (a card keeping the name lich gave it) rather than as
// a title of "".
func TestKiroMetadataTitle(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			"the title Kiro derived from the first prompt",
			`{"session_id":"s1","cwd":"/w","title":"Fix the flaky PTY test","session_state":{}}`,
			"Fix the flaky PTY test",
		},
		{"metadata written before the title", `{"session_id":"s1","cwd":"/w"}`, ""},
		{"a title Kiro filed empty", `{"title":""}`, ""},
		{"a title that is only spaces", `{"title":"   "}`, ""},
		{"surrounding space is trimmed", `{"title":"  Name it  "}`, "Name it"},
		// The file is written by another process, so a read can land mid-write.
		{"caught half-written", `{"session_id":"s1","tit`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.json")
			if err := os.WriteFile(path, []byte(tt.body), 0o600); err != nil {
				t.Fatalf("write session file: %v", err)
			}
			got, ok := kiroMetadataTitle(path)
			if ok != (tt.want != "") {
				t.Fatalf("kiroMetadataTitle = %q, %v, want ok=%v", got, ok, tt.want != "")
			}
			if got != tt.want {
				t.Errorf("kiroMetadataTitle = %q, want %q", got, tt.want)
			}
		})
	}
}

// A conversation whose metadata file is not there yet, or was pruned, is the
// common absence, and it must not reach the card as an error.
func TestKiroMetadataTitleAbsentFile(t *testing.T) {
	if got, ok := kiroMetadataTitle(filepath.Join(t.TempDir(), "absent.json")); ok {
		t.Errorf("kiroMetadataTitle(absent) = %q, true, want a miss", got)
	}
}
