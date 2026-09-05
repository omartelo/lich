package gitutil

import (
	"slices"
	"testing"
)

// TestNoPromptSilencesEveryPrompter pins the four settings: git's own terminal
// prompt, the two askpass hooks and the credential manager each open a dialog
// on their own, and dropping any one of them is a spawn that hangs on a prompt
// nobody can see.
func TestNoPromptSilencesEveryPrompter(t *testing.T) {
	got := NoPrompt([]string{"HOME=/home/x"})
	want := []string{
		"HOME=/home/x",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GCM_INTERACTIVE=never",
	}
	if !slices.Equal(got, want) {
		t.Errorf("NoPrompt = %v, want %v", got, want)
	}
}
