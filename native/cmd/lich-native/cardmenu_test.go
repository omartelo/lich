package main

import (
	"slices"
	"testing"

	"github.com/omartelo/lich/native/lichclient"
	"github.com/omartelo/lich/native/ui"
)

func labels(entries []ui.MenuEntry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Label)
	}
	return out
}

func entry(t *testing.T, entries []ui.MenuEntry, label string) ui.MenuEntry {
	t.Helper()
	i := slices.IndexFunc(entries, func(e ui.MenuEntry) bool { return e.Label == label })
	if i < 0 {
		t.Fatalf("no %q in %v", label, labels(entries))
	}
	return entries[i]
}

func TestCardMenu(t *testing.T) {
	var act sidebarAction
	renamed := false
	sv := &sessionView{Session: lichclient.Session{ID: "s1", Kind: "claude", Color: "teal"}, shown: "/repo"}
	menu := cardMenu(sv, &act, func() { renamed = true })

	if want := []string{"Rename", "Pin", "Color", "Open in", "Pull request", "", "Close session"}; !slices.Equal(labels(menu), want) {
		t.Fatalf("menu %v, want %v", labels(menu), want)
	}
	entry(t, menu, "Rename").OnClick()
	entry(t, menu, "Pin").OnClick()
	entry(t, entry(t, menu, "Color").Sub, "Theme").OnClick()
	entry(t, entry(t, menu, "Open in").Sub, "Terminal").OnClick()
	entry(t, menu, "Close session").OnClick()
	if !renamed || act.pin != "s1" || !act.pinTo || act.color != "s1" || act.colorTo != "" || act.terminalAt != "/repo" || act.close != "s1" {
		t.Fatalf("after the clicks: renamed %v, act %+v", renamed, act)
	}
	if !entry(t, entry(t, menu, "Color").Sub, "Teal").Checked {
		t.Error("the card's own color is not checked")
	}
	if !entry(t, menu, "Pull request").Disabled {
		t.Error("Pull request enabled on a branch without one")
	}

	sv.Kind, sv.Pinned = "shell", true
	menu = cardMenu(sv, &act, func() {})
	if slices.Contains(labels(menu), "Close session") || entry(t, menu, "Unpin").Label != "Unpin" {
		t.Errorf("pinned menu %v: want Unpin and no Close session", labels(menu))
	}
	if slices.Contains(labels(entry(t, menu, "Open in").Sub), "Terminal") {
		t.Error("a shell session offers a terminal at its own directory")
	}
}
