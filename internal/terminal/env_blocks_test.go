package terminal

import (
	"encoding/binary"
	"reflect"
	"testing"
	"unicode/utf16"
)

func TestProcArgsEnvironment(t *testing.T) {
	fixture := append([]byte{3, 0, 0, 0}, []byte("/bin/agent\x00\x00agent\x00CLAUDE_CONFIG_DIR=argument\x00\x00CLAUDE_CONFIG_DIR=/work\x00CLAUDE_SECURESTORAGE_CONFIG_DIR=\x00TOKEN=a=b\x00\x00apple=ignored\x00")...)
	want := map[string]string{"CLAUDE_CONFIG_DIR": "/work", "CLAUDE_SECURESTORAGE_CONFIG_DIR": "", "TOKEN": "a=b"}
	if got := parseProcArgsEnv(fixture); !reflect.DeepEqual(got, want) {
		t.Fatal("procargs did not return precisely the environment fixture")
	}
	for _, data := range [][]byte{nil, {1, 0}, {255, 255, 255, 255}, {0, 0, 0, 0},
		append([]byte{1, 0, 0, 0}, []byte("/bin/agent")...),
		append([]byte{2, 0, 0, 0}, []byte("/bin/agent\x00agent\x00")...),
		append([]byte{1, 0, 0, 0}, []byte("/bin/agent\x00agent\x00BROKEN\x00\x00")...),
		fixture[:len(fixture)-len("\x00apple=ignored\x00")]} {
		if parseProcArgsEnv(data) != nil {
			t.Error("malformed procargs must be unknown")
		}
	}
	empty := append([]byte{1, 0, 0, 0}, []byte("/bin/agent\x00agent\x00\x00")...)
	if got := parseProcArgsEnv(empty); got == nil || len(got) != 0 {
		t.Error("empty environment is readable")
	}
}

func utf16EnvFixture(s string) []byte {
	var data []byte
	for _, unit := range utf16.Encode([]rune(s)) {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return data
}

func TestUTF16Environment(t *testing.T) {
	data := utf16EnvFixture("=C:=C:\\work\x00CLAUDE_CONFIG_DIR=C:\\café😀\x00CLAUDE_SECURESTORAGE_CONFIG_DIR=\x00TOKEN=a=b\x00\x00")
	want := map[string]string{"CLAUDE_CONFIG_DIR": "C:\\café😀", "CLAUDE_SECURESTORAGE_CONFIG_DIR": "", "TOKEN": "a=b"}
	if got := parseUTF16Env(data); !reflect.DeepEqual(got, want) {
		t.Fatal("UTF-16 block did not return precisely the environment fixture")
	}
	for _, bad := range [][]byte{nil, data[:len(data)-1], data[:len(data)-2],
		utf16EnvFixture("BROKEN\x00\x00"), {0, 0xd8, 0, 0}, {0, 0xdc, 0, 0}, {0, 0xd8}} {
		if parseUTF16Env(bad) != nil {
			t.Error("malformed UTF-16 environment must be unknown")
		}
	}
	if got := parseUTF16Env(utf16EnvFixture("\x00\x00")); got == nil || len(got) != 0 {
		t.Error("empty environment is readable")
	}
}
