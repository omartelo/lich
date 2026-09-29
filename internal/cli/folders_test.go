package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/omartelo/lich/internal/spawn"
)

func TestMCPFileSessionPostsItsArgumentsInOrder(t *testing.T) {
	f := newFakeLich(t, `{"id":"s2","project":"lich","label":"auth-fix","folder":"Apps","previous":""}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"file_session","arguments":{"session":"auth-fix","project":"lich","folder":"Apps"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	call := f.only(t)
	want := []any{"s1", "auth-fix", "lich", "Apps"}
	if call.method != "spawn.File" || len(call.args) != len(want) {
		t.Fatalf("call = %s %v, want spawn.File %v", call.method, call.args, want)
	}
	for i := range want {
		if call.args[i] != want[i] {
			t.Errorf("argument %d = %v, want %v", i, call.args[i], want[i])
		}
	}
	if !strings.Contains(text, `"auth-fix" under "Apps"`) {
		t.Errorf("result = %q, want where the session is now", text)
	}
}

// An empty folder is a real request (take the session out), so the tool
// passes it through rather than reading it as missing.
func TestMCPFileSessionIntoAnEmptyFolderTakesItOut(t *testing.T) {
	f := newFakeLich(t, `{"id":"s1","project":"lich","label":"planner","folder":"","previous":"Apps"}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"file_session","arguments":{"folder":""}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	if got := f.only(t).args[3]; got != "" {
		t.Errorf("folder sent as %v, want the empty name", got)
	}
	if !strings.Contains(text, `out of "Apps"`) {
		t.Errorf("result = %q, want the folder it left", text)
	}
}

// Because the empty name takes the session out, a call that left the folder
// out entirely must not be read as one.
func TestMCPFileSessionWithoutAFolderIsRefused(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"file_session","arguments":{"session":"auth-fix"}}}`)

	text, failed := textOf(t, replies[0])
	if !failed || !strings.Contains(text, "name the folder") {
		t.Errorf("result = %q (failed %v), want the missing folder refused", text, failed)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %+v, want nothing sent to lich", f.calls)
	}
}

func TestMCPRenameFolderPostsItsArgumentsInOrder(t *testing.T) {
	f := newFakeLich(t, `{"project":"lich","from":"Apps","to":"Applications","sessions":["a","b"]}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"rename_folder","arguments":{"folder":"Apps","to":"Applications"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	call := f.only(t)
	want := []any{"s1", "", "Apps", "Applications"}
	if call.method != "spawn.RenameFolder" || len(call.args) != len(want) {
		t.Fatalf("call = %s %v, want spawn.RenameFolder %v", call.method, call.args, want)
	}
	for i := range want {
		if call.args[i] != want[i] {
			t.Errorf("argument %d = %v, want %v", i, call.args[i], want[i])
		}
	}
	if !strings.Contains(text, `"a", "b"`) {
		t.Errorf("result = %q, want every session that moved", text)
	}
}

func TestMCPRenameFolderWithoutANewNameIsRefused(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"rename_folder","arguments":{"folder":"Apps"}}}`)

	text, failed := textOf(t, replies[0])
	if !failed || !strings.Contains(text, "new name") {
		t.Errorf("result = %q (failed %v), want the missing name refused", text, failed)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %+v, want nothing sent to lich", f.calls)
	}
}

func TestMCPListFoldersReturnsThemAsJSON(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"list_folders","arguments":{"project":"lich"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	if call := f.only(t); call.method != "spawn.Folders" || call.args[1] != "lich" {
		t.Errorf("call = %s %v, want spawn.Folders for the project named", call.method, call.args)
	}
	if strings.TrimSpace(text) != "[]" {
		t.Errorf("result = %q, want []: an agent should not have to handle null", text)
	}
}

func TestFoldersJSONIsAListEvenWhenThereAreNone(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, stdout, stderr := run(t, f, "folders", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	var folders []spawn.Folder
	if err := json.Unmarshal([]byte(stdout), &folders); err != nil || folders == nil {
		t.Errorf("stdout = %q, want an empty JSON list", stdout)
	}
}

func TestFileTakesAtMostASessionAndAFolder(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, _, stderr := run(t, f, "file", "a", "b", "c")
	if code == 0 || !strings.Contains(stderr, "usage: lich file") {
		t.Errorf("exit = %d, stderr = %q, want the usage line", code, stderr)
	}
}

func TestRenameFolderNeedsBothNames(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, _, stderr := run(t, f, "rename-folder", "Apps")
	if code == 0 || !strings.Contains(stderr, "usage: lich rename-folder") {
		t.Errorf("exit = %d, stderr = %q, want the usage line", code, stderr)
	}
}

func TestFiledTextSaysWhereTheSessionIsNow(t *testing.T) {
	cases := map[string]struct {
		filed spawn.Filed
		want  string
	}{
		"filed":     {spawn.Filed{Label: "a", Folder: "Apps", Previous: "Infra"}, `Filed "a" under "Apps"`},
		"taken out": {spawn.Filed{Label: "a", Previous: "Apps"}, `Took "a" out of "Apps"`},
		"in none":   {spawn.Filed{Label: "a"}, `"a" was in no folder`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := filedText(tc.filed); !strings.Contains(got, tc.want) {
				t.Errorf("filedText = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRefiledTextTakingAFolderApartSaysItIsGone(t *testing.T) {
	got := refiledText(spawn.Refiled{From: "Apps", Sessions: []string{"a", "b"}})
	if !strings.Contains(got, `Took folder "Apps" apart, taking out "a", "b".`) {
		t.Errorf("refiledText = %q", got)
	}
}
