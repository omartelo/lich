package awake

// holdArgv is a logind idle inhibitor held for as long as cat runs. `idle` is
// the one class that means "do not sleep for lack of activity" and nothing
// more, `sleep` would block a suspend the user chose, and it is the class
// GNOME, KDE and hypridle all read before acting on their idle timeout.
func holdArgv() []string {
	return []string{
		"systemd-inhibit", "--what=idle", "--who=lich",
		"--why=a session is working", "--mode=block", "cat",
	}
}
