// Package gitutil holds the small process rules shared by git callers.
package gitutil

// NoPrompt adds every setting needed to stop git and its credential helpers
// from opening a terminal or GUI prompt nobody can answer.
func NoPrompt(env []string) []string {
	return append(env,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GCM_INTERACTIVE=never",
	)
}
