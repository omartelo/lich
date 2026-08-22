package system

// openDefault hands the file to its shell association when no $VISUAL/$EDITOR
// is set. explorer takes the path as a plain argument, which is the point: the
// `cmd /c start` this replaced handed it to a shell that reads & | < > as
// command separators, and Go quotes an argument only when it holds a space — so
// a file named "a&calc.txt", which a checked-out branch is free to carry, ran
// calc. explorer exits 1 even when it worked; nothing waits on it.
func (s *Service) openDefault(full string) error {
	return s.run("explorer", full)
}

// quoteForShell quotes a path for cmd.exe, the shell a Windows session runs.
func (s *Service) quoteForShell(full string) (string, bool) {
	return quoteCmdPath(full)
}

// openURL hands an external URL to the default browser. Deliberately not the
// `cmd /c start` above: cmd parses metacharacters out of its command line before
// start ever sees them, so a `&` in a query string would end the command and run
// what follows it. rundll32 is launched directly, with no shell in between.
func (s *Service) openURL(rawURL string) error {
	return s.run("rundll32", "url.dll,FileProtocolHandler", rawURL)
}

// openFolder shows a directory in File Explorer. Same launcher as openDefault,
// and for the same reason: explorer takes the path as a plain argument, with no
// shell in between to read & | < > out of it.
func (s *Service) openFolder(dir string) error {
	return s.run("explorer", dir)
}
