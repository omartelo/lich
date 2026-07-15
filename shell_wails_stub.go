//go:build nowails

package main

import (
	"log"

	"github.com/omartelo/lich/internal/claudeplugin"
	"github.com/omartelo/lich/internal/fonts"
	"github.com/omartelo/lich/internal/project"
	"github.com/omartelo/lich/internal/store"
	"github.com/omartelo/lich/internal/terminal"
)

// This build carries no Wails (and so no GTK/WebKitGTK) linkage: the Chromium
// shell is the only shell, and main routes there unconditionally.
const wailsShellAvailable = false

// eventFallback: no Wails bridge exists; events wait for the /events socket.
func eventFallback() func(name string, data any) {
	return nil
}

func shellPicker(bool) project.Picker {
	return project.ZenityPicker{}
}

func runWails(*terminal.Service, *project.Service, *fonts.Service, *claudeplugin.Service, *store.Service) {
	log.Fatal("this build has no wails shell (built with -tags nowails); unset LICH_SHELL")
}
