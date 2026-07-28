package system

// openDefault hands the file to the shell's file association when no
// $VISUAL/$EDITOR is set. `start` is a cmd builtin, hence `cmd /c`; the empty
// first argument is start's title slot, which a quoted path would otherwise
// consume.
func (s *Service) openDefault(full string) error {
	return s.run("cmd", "/c", "start", "", full)
}

// openURL hands an external URL to the default browser. Deliberately not the
// `cmd /c start` above: cmd parses metacharacters out of its command line before
// start ever sees them, so a `&` in a query string would end the command and run
// what follows it. rundll32 is launched directly, with no shell in between.
func (s *Service) openURL(rawURL string) error {
	return s.run("rundll32", "url.dll,FileProtocolHandler", rawURL)
}
