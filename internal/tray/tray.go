// Package tray keeps lich in the system tray: the one thing on screen that says
// a lich whose window is closed is still running its sessions, and the way back
// to the window or out of lich from there.
//
// The menu's words are the page's: the interface language lives in the page,
// so the page hands its translated labels over (SetLabels), and the tray comes
// up with the first of them. A lich whose page never loaded shows no tray, and
// has its window, or the error that kept it from opening, on screen instead.
package tray

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// refreshEvery is how often the menu's session count is read again. The count
// only changes when a session is opened or closed, and nothing reads it but a
// person opening the menu.
const refreshEvery = 5 * time.Second

// Labels are the menu's words, in the interface language. Running carries a
// {count} placeholder and is written so it needs no plural form.
type Labels struct {
	Show    string `json:"show"`
	Running string `json:"running"`
	Quit    string `json:"quit"`
}

func (l Labels) running(live int) string {
	return strings.ReplaceAll(l.Running, "{count}", strconv.Itoa(live))
}

// Tray is lich's tray icon and its menu.
type Tray struct {
	mu      sync.Mutex
	labels  Labels
	started bool
	// relabel is set once the menu is up, to rewrite it in new labels.
	relabel func(Labels)
	icons   Icons
	show    func()
	quit    func() error
	live    func() int
	// start puts the icon up; the build-tagged native one, or a test's.
	start func(t *Tray)
}

// Icons are the two encodings the platforms take: PNG for the Linux
// StatusNotifierItem, ICO for the Windows notification area.
type Icons struct {
	PNG []byte
	ICO []byte
}

// New returns a tray that is not up yet. show brings the window up, quit ends
// lich, live counts the sessions with a process running.
func New(icons Icons, show func(), quit func() error, live func() int) *Tray {
	return &Tray{icons: icons, show: show, quit: quit, live: live, start: startNative}
}

// SetLabels gives the menu its words. The first call puts the tray up; a later
// one, after the language changed, rewrites the menu in place.
func (t *Tray) SetLabels(labels Labels) {
	t.mu.Lock()
	t.labels = labels
	first := !t.started
	t.started = true
	relabel := t.relabel
	t.mu.Unlock()
	if first {
		t.start(t)
		return
	}
	if relabel != nil {
		relabel(labels)
	}
}

func (t *Tray) currentLabels() Labels {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.labels
}

func (t *Tray) setRelabel(relabel func(Labels)) {
	t.mu.Lock()
	t.relabel = relabel
	t.mu.Unlock()
}
