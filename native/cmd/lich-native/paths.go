package main

import "regexp"

// homeRoot is frontend/src/lib/paths.ts's HOME_ROOTS.
var homeRoot = regexp.MustCompile(`^(?:/home/[^/]+|/Users/[^/]+|[A-Za-z]:\\Users\\[^\\]+)`)

// displayPath collapses a home directory prefix to "~", as shells render it.
func displayPath(path string) string {
	return homeRoot.ReplaceAllLiteralString(path, "~")
}

// unknownCwd is the path readout when the shell went somewhere the backend
// cannot see into (tmux, ssh, a container): the last local path is a
// directory the user has left. Mirrors env.paths.cwdUnknown.
func unknownCwd(host string) string {
	return "cwd unknown · inside " + host
}
