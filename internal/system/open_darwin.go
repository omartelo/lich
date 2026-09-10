package system

import "github.com/omartelo/lich/internal/shquote"

// openDefault opens the file in the default text editor when no $VISUAL/$EDITOR
// is set. `open -t` targets the editor bound to the Default Editor, not the
// file type's app, so a source file lands in an editor rather than a viewer.
func (s *Service) openDefault(full string) error {
	return s.run("open", "-t", full)
}

// urlOpenArgv is how macOS opens a URL in the default browser. No -t here: that
// flag forces the text editor, which is right for a source file and wrong for a
// link.
func urlOpenArgv(rawURL string) []string {
	return []string{"open", rawURL}
}

// quoteForShell quotes a path for the POSIX shell a macOS session runs.
func (s *Service) quoteForShell(full string) string {
	return shquote.Quote(full)
}

// openFolder shows a directory in Finder. Deliberately without the -t of
// openDefault: that flag forces the default *text editor*, which is what a
// source file wants and what a folder has no use for.
func (s *Service) openFolder(dir string) error {
	return s.run("open", dir)
}
