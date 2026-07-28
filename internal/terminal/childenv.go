package terminal

import "strings"

// appImageVars are injected by the AppImage runtime or AppImageLauncher and
// must not leak into the shell: ARGV0 (the .AppImage's invocation name) makes
// mise/asdf-style shims misread the shell as an invalid shim, and the rest
// are runtime internals a child never needs.
var appImageVars = map[string]bool{
	"ARGV0":              true,
	"APPIMAGE":           true,
	"APPDIR":             true,
	"OWD":                true,
	"TARGET_APPIMAGE":    true,
	"REDIRECT_APPIMAGE":  true,
	"DESKTOPINTEGRATION": true,
}

// childEnv returns env cleaned of everything the AppImage runtime injected, so
// spawned shells inherit the environment lich itself was launched with. Outside
// an AppImage (no APPDIR — the deb/rpm/dev case) env is returned unchanged.
// Inside one, appImageVars are dropped and every value is scrubbed of
// colon-separated path entries under the AppImage mount — our AppRun adds
// none, but wrappers like AppImageLauncher may prepend the mount to path
// lists, and a mount path in a child's PATH dies with the parent. Entries the
// user set survive verbatim, so a pre-existing LD_LIBRARY_PATH keeps working.
func childEnv(env []string) []string {
	appdir := ""
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "APPDIR="); ok {
			appdir = v
			break
		}
	}
	if appdir == "" {
		return env
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		key, value, _ := strings.Cut(kv, "=")
		if appImageVars[key] {
			continue
		}
		scrubbed, changed := scrubPathList(value, appdir)
		switch {
		case changed && scrubbed == "":
			// The variable only pointed into the mount (AppRun's "${VAR:-}"
			// expansion can also leave a dangling empty entry): drop it.
		case changed:
			out = append(out, key+"="+scrubbed)
		default:
			out = append(out, kv)
		}
	}
	return out
}

// scrubPathList drops colon-separated entries of value that live under dir.
// changed reports whether anything was dropped; values without such entries are
// returned untouched, so non-path variables are never rewritten. When only
// empty entries remain the returned value is "".
func scrubPathList(value, dir string) (string, bool) {
	if !strings.Contains(value, dir) {
		return value, false
	}
	var kept []string
	changed, nonEmpty := false, false
	for entry := range strings.SplitSeq(value, ":") {
		if entry == dir || strings.HasPrefix(entry, dir+"/") {
			changed = true
			continue
		}
		if entry != "" {
			nonEmpty = true
		}
		kept = append(kept, entry)
	}
	if !changed {
		return value, false
	}
	if !nonEmpty {
		return "", true
	}
	return strings.Join(kept, ":"), true
}
