//go:build darwin

package quota

import (
	"errors"
	"slices"
	"testing"
)

func TestCursorKeychainItemAsksForCursorAgentsItem(t *testing.T) {
	var asked []string
	got := cursorKeychainItem(func(args ...string) ([]byte, error) {
		asked = args
		return []byte("tok-cursor\n"), nil
	})
	if got != "tok-cursor" {
		t.Errorf("token = %q, want the item's password trimmed", got)
	}
	want := []string{"find-generic-password", "-a", "cursor-user", "-w", "-s", "cursor-access-token"}
	if !slices.Equal(asked, want) {
		t.Errorf("security args = %q, want %q", asked, want)
	}
}

func TestCursorKeychainFailureReadsAsNoItem(t *testing.T) {
	got := cursorKeychainItem(func(...string) ([]byte, error) { return nil, errors.New("locked") })
	if got != "" {
		t.Errorf("token = %q, want none from a Keychain that refused", got)
	}
}
