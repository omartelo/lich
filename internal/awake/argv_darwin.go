package awake

// holdArgv is caffeinate's idle-sleep assertion (-i: PreventUserIdleSystemSleep)
// held for as long as cat runs. The display still sleeps: that is -d, and a
// working agent has no use for a lit screen.
func holdArgv() []string {
	return []string{"caffeinate", "-i", "cat"}
}
