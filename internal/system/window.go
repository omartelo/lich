package system

// The backend outlives its window: closing the window leaves every session
// running, and these two calls are how anyone else gets the window back or
// ends lich for good. A second launch of lich asks for ShowWindow, and
// `lich quit` asks for Quit.

// SetShowWindow wires how lich opens its window, or brings the open one to the
// front. Called at startup.
func (s *Service) SetShowWindow(show func()) {
	s.showWindow = show
}

// SetCloseWindow wires how lich closes its window and keeps running. Called at
// startup.
func (s *Service) SetCloseWindow(closeWindow func() error) {
	s.closeWindow = closeWindow
}

// SetQuit wires how lich ends. Called at startup.
func (s *Service) SetQuit(quit func() error) {
	s.quit = quit
}

// ShowWindowOptions is one ShowWindow call. It carries nothing yet; it is an
// object so a newer launch can still reach an older running lich the day it
// does (the CLI's options-object rule).
type ShowWindowOptions struct{}

// ShowWindow opens lich's window on the running backend when none is open, and
// brings the open one forward otherwise. It returns as soon as the window is
// asked for, not when it is on screen.
func (s *Service) ShowWindow(ShowWindowOptions) {
	s.showWindow()
}

// CloseWindowOptions is one CloseWindow call, an object for the reason
// ShowWindowOptions is.
type CloseWindowOptions struct{}

// CloseWindow closes lich's window and leaves lich running with its sessions:
// the page's answer when the user chose to keep it running in the background.
// The page is not asked again on the way out.
func (s *Service) CloseWindow(CloseWindowOptions) error {
	return s.closeWindow()
}

// QuitOptions is one Quit call, an object for the reason ShowWindowOptions is.
type QuitOptions struct{}

// Quit ends lich: the window closes, every session's process ends with it, and
// the process exits. It returns once the exit is under way, so the reply can be
// lost to the exit it announces.
func (s *Service) Quit(QuitOptions) error {
	return s.quit()
}
