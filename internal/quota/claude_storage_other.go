//go:build !darwin

package quota

func readClaudeCredentials(a Account) (claudeCredentials, string) {
	return readClaudeFile(a)
}
