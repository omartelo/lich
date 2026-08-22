package system

import "github.com/omartelo/lich/internal/shquote"

// openDefault hands the file to the desktop's default handler when no
// $VISUAL/$EDITOR is set. xdg-open resolves the user's chosen application.
func (s *Service) openDefault(full string) error {
	return s.run("xdg-open", full)
}

// openURL hands an external URL to the desktop's default browser — the same
// resolver, and the reason a link opens outside the app window.
func (s *Service) openURL(rawURL string) error {
	return s.run("xdg-open", rawURL)
}

// quoteForShell quotes a path for the POSIX shell a Linux session runs. Every
// path can be expressed, so this never refuses.
func (s *Service) quoteForShell(full string) (string, bool) {
	return shquote.Quote(full), true
}

// openFolder shows a directory in the desktop's file manager. Same resolver as
// openDefault: xdg-open already answers a directory with the file manager
// rather than an editor.
func (s *Service) openFolder(dir string) error {
	return s.run("xdg-open", dir)
}
