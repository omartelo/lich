package system

import "github.com/omartelo/lich/internal/shquote"

// openDefault hands the file to its shell association when no $VISUAL/$EDITOR
// is set. explorer takes the path as a plain argument, which is the point: the
// `cmd /c start` this replaced handed it to a shell that reads & | < > as
// command separators, and Go quotes an argument only when it holds a space — so
// a file named "a&calc.txt", which a checked-out branch is free to carry, ran
// calc. explorer exits 1 even when it worked; nothing waits on it.
func (s *Service) openDefault(full string) error {
	return s.run("explorer", full)
}

// quoteForShell quotes a path for PowerShell, the shell a Windows session runs
// (internal/terminal, windowsShells). Every path can be expressed, so this never
// refuses — cmd.exe, which lich no longer opens a session in, could not express
// a path holding a double quote or a %VAR% and had to hand those to the default
// handler instead.
func (s *Service) quoteForShell(full string) string {
	return shquote.QuotePwsh(full)
}

// urlOpenArgv is how Windows opens a URL in the default browser. Deliberately
// not the `cmd /c start` above: cmd parses metacharacters out of its command
// line before start ever sees them, so a `&` in a query string would end the
// command and run what follows it. rundll32 is launched directly, with no shell
// in between.
func urlOpenArgv(rawURL string) []string {
	return []string{"rundll32", "url.dll,FileProtocolHandler", rawURL}
}

// openFolder shows a directory in File Explorer. Same launcher as openDefault,
// and for the same reason: explorer takes the path as a plain argument, with no
// shell in between to read & | < > out of it.
func (s *Service) openFolder(dir string) error {
	return s.run("explorer", dir)
}
