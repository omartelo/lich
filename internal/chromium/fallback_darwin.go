package chromium

// TabFallback: a Mac whose window is missing or dies at startup still opens
// lich, as a plain tab in the default browser (main.go's openWithoutWindow). macOS alone keeps that
// rung: Linux and Windows ship the window in every package, so its absence
// there is a broken install to report, not a machine to serve anyway.
const TabFallback = true
