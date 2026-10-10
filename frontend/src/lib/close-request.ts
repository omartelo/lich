// Closing lich's window no longer quits lich: its sessions keep running in the
// background. What the user wants instead is asked when they close it, and the
// answer can be kept (CLOSE_ACTION_SETTING_KEY).
//
// The page holds the window open by cancelling beforeunload, which only lich's
// own window answers without Chromium's leave-page dialog: lich-shell keeps a
// close the user made, lets a reload and a close lich made through, and then
// tells the page with CLOSE_REQUESTED_EVENT (shell/src/main.rs).

export const CLOSE_ACTION_SETTING_KEY = "window.closeAction"

export const CLOSE_ACTIONS = ["ask", "background", "quit"] as const

export type CloseAction = (typeof CLOSE_ACTIONS)[number]

// The event lich-shell dispatches on the window once it kept a close the user
// asked for (CLOSE_REQUESTED_SCRIPT in shell/src/main.rs).
export const CLOSE_REQUESTED_EVENT = "lich-close-requested"

// closeActionOf reads the stored choice; nothing stored, or a value from a
// newer lich, asks.
export function closeActionOf(stored: string): CloseAction {
  return (CLOSE_ACTIONS as readonly string[]).includes(stored) ? (stored as CloseAction) : "ask"
}

// inLichWindow is whether the page is in lich's own window, which marks its URL
// (main.go, pageOn). The macOS tab fallback is a plain browser tab: holding its
// close would only bring up the browser's own leave-page dialog.
export function inLichWindow(href: string): boolean {
  return new URL(href).searchParams.get("shell") === "1"
}

// holdsClose is whether a close has to wait for the page: everything but a kept
// "keep running", which the window may simply do.
export function holdsClose(action: CloseAction): boolean {
  return action !== "background"
}

// What a close the page held turns into: quitting outright with no session to
// keep, or when that is the kept answer; asking otherwise.
export type CloseOutcome = "quit" | "ask"

export function closeOutcome(action: CloseAction, liveSessions: number): CloseOutcome {
  return liveSessions === 0 || action === "quit" ? "quit" : "ask"
}
