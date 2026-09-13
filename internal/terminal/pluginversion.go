package terminal

import (
	"log/slog"
	"sync"

	"github.com/omartelo/lich/internal/agentplugin"
)

// pluginVersionHeader names the plugin release a hook report comes from
// (docs/hooks/README.md, Shared transport).
const pluginVersionHeader = "X-Lich-Plugin"

// pluginEventName carries a session whose hooks report from a plugin release
// outside the range this lich speaks ({id, version}). The window answers it by
// asking for the plugin status again, which is where the fix is offered: an
// install moved by the harness's own marketplace shows up here first.
const pluginEventName = "plugin-incompatible"

type pluginEvent struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// hookBody is what every hook request carries: the session it reports on.
type hookBody interface {
	session() string
}

func (r hookRequest) session() string    { return r.SessionID }
func (r startRequest) session() string   { return r.SessionID }
func (r titleRequest) session() string   { return r.SessionID }
func (r touchedRequest) session() string { return r.SessionID }

// pluginVersions remembers the plugin release each session's hooks last named,
// so a mismatch is warned about once per session and release rather than once
// per report. A session's hooks can change release mid-life: a harness that
// reloads its plugins, or a PTY respawned after an update.
type pluginVersions struct {
	mu   sync.Mutex
	byID map[string]string
	// onIncompatible is told about a session newly reporting from a release
	// outside the range. Wired after construction, like transport.fallback.
	onIncompatible func(id, version string)
}

// note records the release a session's report named. An absent header is a
// plugin older than the header, and so older than the floor.
func (p *pluginVersions) note(id, version string) {
	p.mu.Lock()
	if p.byID == nil {
		p.byID = map[string]string{}
	}
	prev, seen := p.byID[id]
	p.byID[id] = version
	fn := p.onIncompatible
	p.mu.Unlock()

	if (seen && prev == version) || agentplugin.Compatible(version) {
		return
	}
	slog.Warn("hook: plugin outside the supported range", "session", id, "plugin", pluginLabel(version),
		"floor", agentplugin.PluginVersionFloor, "ceiling", agentplugin.PluginVersionCeiling)
	if fn != nil {
		fn(id, version)
	}
}

func (p *pluginVersions) setOnIncompatible(fn func(id, version string)) {
	p.mu.Lock()
	p.onIncompatible = fn
	p.mu.Unlock()
}

func (p *pluginVersions) forget(id string) {
	p.mu.Lock()
	delete(p.byID, id)
	p.mu.Unlock()
}

// pluginLabel is how a report's plugin release reads in a log line.
func pluginLabel(version string) string {
	if version == "" {
		return "absent"
	}
	return version
}
