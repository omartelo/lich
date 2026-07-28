package system

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
