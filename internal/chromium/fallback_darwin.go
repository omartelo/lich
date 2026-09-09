package chromium

// TabFallback: a Mac with no window of its own — an Intel bundle, or an Apple
// Silicon window that dies at startup — still opens lich, as a plain tab in
// the default browser (main.go's openWithoutWindow). macOS alone keeps that
// rung: Linux and Windows ship the window in every package, so its absence
// there is a broken install to report, not a machine to serve anyway.
const TabFallback = true
