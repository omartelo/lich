package tray

// startNative puts nothing up on macOS: its menu-bar item needs cgo, and lich is
// built without it (docs/ceilings.md).
func startNative(*Tray) {}
