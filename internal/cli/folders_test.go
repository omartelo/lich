package cli

import (
	"encoding/json"
	"reflect"
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
	if call.method != "spawn.File" {
		t.Fatalf("method = %q, want spawn.File", call.method)
	}
	want := spawn.FileOptions{From: "s1", Target: "auth-fix", Project: "lich", Folder: "Apps"}
	if got := optionsOf[spawn.FileOptions](t, call); got != want {
		t.Errorf("options = %+v, want %+v", got, want)
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
	if got := optionsOf[spawn.FileOptions](t, f.only(t)).Folder; got != "" {
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
	if call.method != "spawn.RenameFolder" {
		t.Fatalf("method = %q, want spawn.RenameFolder", call.method)
	}
	want := spawn.RenameFolderOptions{From: "s1", Folder: "Apps", To: "Applications"}
	if got := optionsOf[spawn.RenameFolderOptions](t, call); got != want {
		t.Errorf("options = %+v, want %+v", got, want)
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

func TestMCPColorFolderPostsItsArgumentsInOrder(t *testing.T) {
	f := newFakeLich(t, `{"project":"lich","folder":"Apps","color":"teal","sessions":["a","b"]}`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"color_folder","arguments":{"folder":"Apps","color":"teal"}}}`)

	text, failed := textOf(t, replies[0])
	if failed {
		t.Fatalf("tool reported a failure: %s", text)
	}
	call := f.only(t)
	if call.method != "spawn.ColorFolder" {
		t.Fatalf("method = %q, want spawn.ColorFolder", call.method)
	}
	want := spawn.ColorFolderOptions{From: "s1", Folder: "Apps", Color: "teal"}
	if got := optionsOf[spawn.ColorFolderOptions](t, call); got != want {
		t.Errorf("options = %+v, want %+v", got, want)
	}
	if !strings.Contains(text, `"a", "b"`) {
		t.Errorf("result = %q, want every session painted", text)
	}
}

// An empty color clears the folder's, so a call that left the color out must
// not be read as one.
func TestMCPColorFolderWithoutAColorIsRefused(t *testing.T) {
	f := newFakeLich(t, `null`)

	replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
		{"name":"color_folder","arguments":{"folder":"Apps"}}}`)

	text, failed := textOf(t, replies[0])
	if !failed || !strings.Contains(text, "name the color") {
		t.Errorf("result = %q (failed %v), want the missing color refused", text, failed)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %+v, want nothing sent to lich", f.calls)
	}
}

func TestColorFolderNeedsAFolderAndAColor(t *testing.T) {
	f := newFakeLich(t, `null`)

	code, _, stderr := run(t, f, "color-folder", "Apps")
	if code == 0 || !strings.Contains(stderr, "usage: lich color-folder") {
		t.Errorf("exit = %d, stderr = %q, want the usage line", code, stderr)
	}
}

func TestColoredTextClearingSaysSo(t *testing.T) {
	got := coloredText(spawn.Colored{Folder: "Apps", Sessions: []string{"a", "b"}})
	if !strings.Contains(got, `Cleared the color of "a", "b" in folder "Apps".`) {
		t.Errorf("coloredText = %q", got)
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
	if call := f.only(t); call.method != "spawn.Folders" || optionsOf[spawn.FoldersOptions](t, call).Project != "lich" {
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

func TestFoldersWithNoneSaysSo(t *testing.T) {
	f := newFakeLich(t, `[]`)

	code, stdout, stderr := run(t, f, "folders")
	if code != 0 || strings.TrimSpace(stdout) != "No folders." {
		t.Errorf("exit = %d, stdout = %q, stderr = %q, want the plain answer", code, stdout, stderr)
	}
}

func TestFoldersTakesNoArguments(t *testing.T) {
	f := newFakeLich(t, `[]`)

	code, _, stderr := run(t, f, "folders", "Apps")
	if code == 0 || !strings.Contains(stderr, "usage: lich folders") {
		t.Errorf("exit = %d, stderr = %q, want the usage line", code, stderr)
	}
	if len(f.calls) != 0 {
		t.Errorf("calls = %+v, want nothing sent to lich", f.calls)
	}
}

// --json hands a script the whole answer, not the sentence the text form prints.
func TestFileAndRenameFolderPrintJSON(t *testing.T) {
	cases := map[string]struct {
		body string
		args []string
		into any
	}{
		"file": {
			`{"id":"s2","project":"lich","label":"auth-fix","folder":"Apps","previous":"Infra"}`,
			[]string{"file", "--json", "auth-fix", "Apps"}, &spawn.Filed{},
		},
		"rename-folder": {
			`{"project":"lich","from":"Apps","to":"Applications","sessions":["auth-fix"]}`,
			[]string{"rename-folder", "--json", "Apps", "Applications"}, &spawn.Refiled{},
		},
		"color-folder": {
			`{"project":"lich","folder":"Apps","color":"teal","sessions":["auth-fix"]}`,
			[]string{"color-folder", "--json", "Apps", "teal"}, &spawn.Colored{},
		},
	}
	want := map[string]any{
		"file":          &spawn.Filed{ID: "s2", Project: "lich", Label: "auth-fix", Folder: "Apps", Previous: "Infra"},
		"rename-folder": &spawn.Refiled{Project: "lich", From: "Apps", To: "Applications", Sessions: []string{"auth-fix"}},
		"color-folder":  &spawn.Colored{Project: "lich", Folder: "Apps", Color: "teal", Sessions: []string{"auth-fix"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeLich(t, tc.body)

			code, stdout, stderr := run(t, f, tc.args...)
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %q", code, stderr)
			}
			if err := json.Unmarshal([]byte(stdout), tc.into); err != nil {
				t.Fatalf("stdout = %q is not JSON: %v", stdout, err)
			}
			if !reflect.DeepEqual(tc.into, want[name]) {
				t.Errorf("printed %+v, want %+v", tc.into, want[name])
			}
		})
	}
}

// A refusal from lich reaches the caller as a failed exit with lich's own words,
// on every folder command.
func TestFolderCommandsReportWhatLichRefused(t *testing.T) {
	for _, args := range [][]string{
		{"folders"},
		{"file", "Apps"},
		{"rename-folder", "Apps", "Applications"},
		{"color-folder", "Apps", "teal"},
	} {
		t.Run(args[0], func(t *testing.T) {
			f := newFakeLich(t, `{"error":"no folder named \"Apps\" in lich"}`)
			f.status = 500

			code, _, stderr := run(t, f, args...)
			if code == 0 || !strings.Contains(stderr, `no folder named "Apps"`) {
				t.Errorf("exit = %d, stderr = %q, want lich's refusal", code, stderr)
			}
		})
	}
}

// The same refusal, reaching an agent through the MCP tools, is a failed tool
// result carrying lich's words rather than a result that reads as success.
func TestMCPFolderToolsReportWhatLichRefused(t *testing.T) {
	for name, arguments := range map[string]string{
		"list_folders":  `{}`,
		"file_session":  `{"folder":"Apps"}`,
		"rename_folder": `{"folder":"Apps","to":"Applications"}`,
		"color_folder":  `{"folder":"Apps","color":"teal"}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFakeLich(t, `{"error":"no folder named \"Apps\" in lich"}`)
			f.status = 500

			replies := speak(t, f, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":
				{"name":"`+name+`","arguments":`+arguments+`}}`)

			text, failed := textOf(t, replies[0])
			if !failed || !strings.Contains(text, `no folder named "Apps"`) {
				t.Errorf("result = %q (failed %v), want lich's refusal", text, failed)
			}
		})
	}
}
