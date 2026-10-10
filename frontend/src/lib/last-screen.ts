import { Store } from "@/lib/rpc"

const GLOBAL_SCOPE = ""

// LAST_SCREEN_SETTING_KEY holds the screen the window was on, so the next window
// opens there: a reopened one on a lich that kept running, and the first one of
// a fresh launch alike. It lives in the store rather than the page's storage,
// which Chromium writes lazily and loses to a window closed seconds later.
export const LAST_SCREEN_SETTING_KEY = "window.lastScreen"

const PROJECT_SCREEN = /^\/projects\/([^/]+)(?:\/.*)?$/

// resumableScreen is the saved screen when it still has somewhere to land: a
// project screen of a project that is open. Anything else, Home included, is
// null, and the window opens on Home as it would have.
export function resumableScreen(saved: string, openProjectIds: readonly string[]): string | null {
  const projectId = PROJECT_SCREEN.exec(saved)?.[1]
  if (!projectId || !openProjectIds.includes(projectId)) {
    return null
  }
  return saved
}

export function loadLastScreen(): Promise<string> {
  return Store.GetSetting(LAST_SCREEN_SETTING_KEY, GLOBAL_SCOPE)
}

export function saveLastScreen(path: string): Promise<null> {
  return Store.SetSetting(LAST_SCREEN_SETTING_KEY, GLOBAL_SCOPE, path)
}
