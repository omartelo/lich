package system

import "github.com/omartelo/lich/internal/shquote"

// openDefault opens the file in the default text editor when no $VISUAL/$EDITOR
// is set. `open -t` targets the editor bound to the Default Editor, not the
// file type's app, so a source file lands in an editor rather than a viewer.
func (s *Service) openDefault(full string) error {
	return s.run("open", "-t", full)
}

// openURL hands an external URL to the default browser. No -t here: that flag
// forces the text editor, which is right for a source file and wrong for a link.
func (s *Service) openURL(rawURL string) error {
	return s.run("open", rawURL)
}

// quoteForShell quotes a path for the POSIX shell a macOS session runs. Every
// path can be expressed, so this never refuses.
func (s *Service) quoteForShell(full string) (string, bool) {
	return shquote.Quote(full), true
}
