// The OS lich itself runs on. navigator.platform is deprecated, but it is what
// the app already read — in three places, each with its own spelling and its own
// guard — and the three must never disagree about which chord a user is shown.
// Read once here instead: "Win32" on Windows Chromium, "MacIntel" on macOS.
const platform = typeof navigator === "undefined" ? "" : navigator.platform.toLowerCase()

export const isMac = platform.includes("mac")
export const isWindows = platform.includes("win")
