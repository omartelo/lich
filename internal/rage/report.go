package rage

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// reported is every environment variable the bundle carries besides LICH_*.
// An allowlist, not a filter: a machine's environment holds cloud credentials,
// provider API keys and whatever the user exports in their rc, and none of that
// belongs in an archive headed for a public issue. These are the ones that
// answer a lich bug — which shell resolved the env, which desktop the window
// opened on, which PATH the provider detection scanned.
var reported = []string{
	"APPDATA", "BROWSER", "DESKTOP_SESSION", "DISPLAY", "EDITOR",
	"GH_CONFIG_DIR", "GH_TOKEN", "GITHUB_TOKEN", "LANG", "PATH",
	"PROCESSOR_ARCHITECTURE", "SHELL", "TERM", "TERM_PROGRAM", "VISUAL",
	"WAYLAND_DISPLAY", "XDG_CURRENT_DESKTOP", "XDG_SESSION_TYPE",
}

// secretish matches the names whose value is never printed, only its presence.
// It runs over the allowlist above and over LICH_*, so a false positive costs a
// value the reporter can still name by hand.
var secretish = regexp.MustCompile(`(?i)token|key|secret|password|passwd|credential`)

// tokenQuery masks the loopback token wherever it was written as a query
// parameter, which is the one shape it takes outside the environment. It backs
// up the exact-value scrub for a token from an earlier run: that instance's
// value is gone, its log line is not.
var tokenQuery = regexp.MustCompile(`(?i)(token=)[^\s&"'` + "`" + `]+`)

// redacted is what replaces a masked value, spelled so a reader can grep the
// bundle for what was removed.
const redacted = "<REDACTED>"

// render writes report.md — the page a maintainer reads first, so the facts
// that decide whether a report is actionable come before anything else.
func render(r Report) string {
	var b strings.Builder
	b.WriteString("# lich bug report\n\n")
	b.WriteString("Collected by `lich rage`. Nothing here was uploaded — read it before you attach it.\n\n")

	b.WriteString("| Fact | Value |\n|---|---|\n")
	row(&b, "lich", r.Version)
	row(&b, "build", buildLine(r))
	row(&b, "platform", r.Platform)
	row(&b, "instance", r.Instance)
	row(&b, "config dir", r.ConfigDir)
	row(&b, "log", logLine(r.LogPath))
	row(&b, "browser", r.Browser)

	b.WriteString("\n## Providers\n\n")
	b.WriteString("| Provider | On PATH | Resolved to |\n|---|---|---|\n")
	for _, p := range r.Providers {
		fmt.Fprintf(&b, "| %s | %s | %s |\n", p.Name, yesNo(p.Installed), dash(p.Path))
	}
	if len(r.Providers) == 0 {
		b.WriteString("| — | — | detection failed |\n")
	}

	b.WriteString("\n## lich plugin\n\n")
	b.WriteString("| Provider | CLI | Installed | Version | Latest |\n|---|---|---|---|---|\n")
	for _, p := range r.Plugin {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			p.Name, yesNo(p.Available), yesNo(p.Installed),
			dash(p.InstalledVersion), dash(p.LatestVersion))
	}
	if len(r.Plugin) == 0 {
		b.WriteString("| — | — | — | — | — |\n")
	}

	b.WriteString("\n## Config directory\n\n")
	b.WriteString("| Entry | Size |\n|---|---|\n")
	for _, e := range r.Entries {
		name := e.Name
		size := humanSize(e.Size)
		if e.Dir {
			name += "/"
			size = "directory"
		}
		fmt.Fprintf(&b, "| %s | %s |\n", name, size)
	}
	if len(r.Entries) == 0 {
		b.WriteString("| — | the config directory could not be read |\n")
	}

	b.WriteString(notice)
	return b.String()
}

// notice closes the report with what the bundle holds and what it never does.
// It is addressed to the reporter, not to the maintainer: they are the one
// deciding whether to attach this to a public issue.
const notice = `
## What this bundle carries

- ` + "`report.md`" + ` — this page.
- ` + "`env.txt`" + ` — the LICH_* variables and a fixed list of shell/desktop ones.
  Anything named like a token, key, secret or password is reported as set or
  unset, never by value.
- ` + "`logs/`" + ` — the log file and the rotated generation before it, each
  capped at its last 4 MiB. They hold absolute paths, project and branch names,
  and your gh login.

Never collected: your Chromium profile, the workspace database, and any
conversation transcript — lich stores none. The loopback token is masked
wherever it appeared.
`

func row(b *strings.Builder, name, value string) {
	fmt.Fprintf(b, "| %s | %s |\n", name, dash(value))
}

// buildLine names the toolchain and, for a binary built from a checkout, the
// commit — the only thing that identifies a build whose version is "dev".
func buildLine(r Report) string {
	if r.Revision == "" {
		return r.GoVersion
	}
	return r.GoVersion + ", revision " + r.Revision
}

func logLine(path string) string {
	if path == "" {
		return "none — lich logged to stderr only"
	}
	return path
}

// renderEnv writes env.txt: every allowlisted name, then whatever LICH_*
// variables this process was launched with, values masked by name and scrubbed
// of the live token.
func renderEnv(environ []string, token string) string {
	values := make(map[string]string, len(environ))
	var lich []string
	for _, kv := range environ {
		key, value, found := strings.Cut(kv, "=")
		if !found {
			continue
		}
		values[key] = value
		if strings.HasPrefix(key, "LICH_") {
			lich = append(lich, key)
		}
	}
	sort.Strings(lich)

	var b strings.Builder
	b.WriteString("# Values are masked by name: anything token/key/secret/password-shaped\n")
	b.WriteString("# is reported as <SET> or <UNSET> only.\n\n")
	for _, key := range reported {
		writeEnvLine(&b, key, values, token)
	}
	b.WriteString("\n")
	for _, key := range lich {
		writeEnvLine(&b, key, values, token)
	}
	return b.String()
}

func writeEnvLine(b *strings.Builder, key string, values map[string]string, token string) {
	value, set := values[key]
	switch {
	case secretish.MatchString(key):
		value = "<UNSET>"
		if set {
			value = "<SET>"
		}
	case !set:
		value = "<UNSET>"
	default:
		value = string(scrub([]byte(value), token))
	}
	fmt.Fprintf(b, "%s=%s\n", key, value)
}

// scrub masks the loopback token in bytes headed for the bundle. The exact
// value covers the running instance, whose token is known; the query form
// covers every earlier one, whose value is not. A short token is ignored —
// masking a two-character string would gut the log.
func scrub(data []byte, token string) []byte {
	if len(token) >= 8 {
		data = []byte(strings.ReplaceAll(string(data), token, redacted))
	}
	return tokenQuery.ReplaceAll(data, []byte("${1}"+redacted))
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// humanSize renders a byte count for a human reading a table, not for a
// machine: one decimal, binary units.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value, exp := float64(n)/unit, 0
	for value >= unit && exp < 2 {
		value /= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", value, [...]string{"KiB", "MiB", "GiB"}[exp])
}
