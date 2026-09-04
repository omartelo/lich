import { useEffect, useRef, useState } from "react"
import { HashRouter, Outlet, Route, Routes, useLocation, useMatch } from "react-router-dom"
import { SettingsProvider, useSettings } from "@/providers/settings"
import { useHotkey } from "@/lib/use-hotkey"
import { parseBoolPref, readPref, writePref } from "@/lib/prefs"
import { ProjectsProvider, useProjects } from "@/providers/projects"
import { activeSessionId, sessionsOf } from "@/lib/session/sessions"
import { usePanes } from "@/lib/session/use-panes"
import {
  requestSessionIntent,
  requestWorktreeDialog,
  type SessionIntent,
} from "@/lib/use-sidebar-intent"
import { ProjectTabs } from "@/components/tabs/ProjectTabs"
import { SessionSidebar } from "@/components/sidebar/SessionSidebar"
import { SidebarRail } from "@/components/sidebar/SidebarRail"
import { TerminalHost } from "@/components/TerminalHost"
import { ErrorBoundary } from "@/components/common/ErrorBoundary"
import { RightDock, type DockTab } from "@/components/dock/RightDock"
import { FooterBar } from "@/components/FooterBar"
import { Home } from "@/components/Home"
import { EmptySessions } from "@/components/EmptySessions"
import { Settings } from "@/components/settings/Settings"
import { Pulls } from "@/components/pulls/Pulls"
import { Toaster } from "@/components/ui/sonner"
import { AgentPluginGate } from "@/components/AgentPluginGate"
import { AppUpdateGate } from "@/components/AppUpdateGate"
import { PatchNotesGate } from "@/components/PatchNotesGate"
import { GitMissingGate } from "@/components/GitMissingGate"
import { ProviderSetupGate } from "@/components/ProviderSetupGate"
import { UncleanExitGate } from "@/components/UncleanExitGate"
import { CommandPalette } from "@/components/CommandPalette"
import { ShortcutsOverlay } from "@/components/ShortcutsOverlay"

// Named here like every other `lich.*` pref rather than spelled at the call
// site. Unlike the dock — opened for a task and closed after it — a hidden
// sidebar is a layout choice, so it survives a restart.
const SIDEBAR_KEY = "lich.sidebar.open"

// Layout is persistent across navigation: the project tabs, session sidebar and
// TerminalHost stay mounted while the Outlet swaps screens (Home, Settings) on
// top of the terminals.
function Layout() {
  const { hotkeys } = useSettings()
  const { sessions } = useProjects()
  const match = useMatch("/projects/:projectId/*")
  const location = useLocation()
  const projectId = match?.params.projectId ?? ""
  const [dock, setDock] = useState<DockTab | null>(null)
  const [sidebar, setSidebar] = useState(() => parseBoolPref(readPref(SIDEBAR_KEY), true))
  const showSidebar = (open: boolean) => {
    setSidebar(open)
    writePref(SIDEBAR_KEY, open)
  }
  const toggleSidebar = () => showSidebar(!sidebar)
  useHotkey(hotkeys.toggleSidebar, toggleSidebar)
  // Both of these act on sidebar chrome — the worktree dialog, a card's rename
  // field — and the rail carries neither, so a collapsed sidebar is opened
  // first rather than letting the chord quietly do nothing. The request is a
  // pending flag for exactly that reason: its consumer mounts a render later
  // (use-sidebar-intent).
  useHotkey(hotkeys.newWorktree, () => {
    if (!projectId) return false
    showSidebar(true)
    requestWorktreeDialog(projectId)
  })
  // The card actions all aim at the same card — the active session's — so they
  // share one route to it. A shortcut is declined wherever that card offers no
  // such action, rather than being swallowed for nothing.
  const active = sessionsOf(sessions, projectId).find(
    (session) => session.id === activeSessionId(sessions, projectId),
  )
  const cardAction = (intent: SessionIntent) => {
    if (!active) return false
    showSidebar(true)
    requestSessionIntent(active.id, intent)
  }
  useHotkey(hotkeys.renameSession, () => cardAction("rename"))
  // A pinned card shows no × at all — closing is what the pin withholds — so
  // the shortcut withholds it too.
  useHotkey(hotkeys.closeSession, () => !active?.pinned && cardAction("close"))
  useHotkey(hotkeys.togglePin, () => cardAction("pin"))
  useHotkey(hotkeys.openTerminal, () => cardAction("terminal"))
  // Delegating writes the request at the session's own prompt, and the thing
  // reading a terminal's prompt is a shell: it would run the line as a command.
  useHotkey(hotkeys.delegate, () => active?.kind !== "shell" && cardAction("delegate"))
  const toggleDock = (tab: DockTab) => setDock((cur) => (cur === tab ? null : tab))
  // The shortcut toggles the dock as a whole, so it reopens on the tab it was
  // last showing — the two tabs have their own footer buttons.
  const lastTab = useRef<DockTab>("files")
  useEffect(() => {
    if (dock) {
      lastTab.current = dock
    }
  }, [dock])
  useHotkey(hotkeys.toggleDock, () => setDock((cur) => (cur ? null : lastTab.current)))
  // Nothing left to show, no room left to show it in, or no second pane to move
  // the cursor to: each declines rather than being swallowed for nothing, the
  // rule every card shortcut above follows.
  const panes = usePanes(projectId)
  useHotkey(hotkeys.splitBeside, () => {
    if (!panes.add()) {
      return false
    }
  })
  useHotkey(hotkeys.otherPane, () => panes.split && panes.focusStep(1))
  return (
    <div className="flex h-screen w-screen flex-col bg-background">
      <ProjectTabs />
      <div className="flex flex-1 overflow-hidden">
        {/* Collapsed is a rail, never nothing: the status rings are what a list
            of running agents is read for, and hiding them to win 12rem of
            terminal is a trade the width alone does not pay for. */}
        {sidebar ? (
          <SessionSidebar onCollapse={toggleSidebar} />
        ) : (
          <SidebarRail onExpand={toggleSidebar} />
        )}
        <main className="flex flex-1 flex-col overflow-hidden">
          {/* relative: RightDock overlays this area when in full screen. */}
          <div className="relative flex flex-1 overflow-hidden">
            <div className="relative flex-1 overflow-hidden">
              {/* TerminalHost is outside on purpose — unmounting it destroys
                  every session's xterm, and no remount brings those back
                  (docs/ceilings.md). The screens over it are what is caught:
                  the fallback takes the same overlay they do, and the route
                  resets it, so navigating away from a blown screen is enough. */}
              <TerminalHost />
              <ErrorBoundary
                label="This screen"
                resetKey={location.pathname}
                className="absolute inset-0 z-10"
              >
                <Outlet />
              </ErrorBoundary>
            </div>
            {dock && (
              <ErrorBoundary label="The panel" className="w-80 shrink-0 border-l border-border">
                <RightDock tab={dock} onTab={setDock} onClose={() => setDock(null)} />
              </ErrorBoundary>
            )}
          </div>
          <FooterBar dock={dock} onDock={toggleDock} />
        </main>
      </div>
    </div>
  )
}

function App() {
  // A drop that misses a terminal must do nothing. The browser's default action
  // for a file is to navigate to it, which in the --app window replaces lich
  // with a file viewer — sessions keep running, but the only way back is a
  // reload. Terminals stop the event before it reaches here (TerminalView).
  useEffect(() => {
    const swallow = (event: DragEvent) => event.preventDefault()
    window.addEventListener("dragover", swallow)
    window.addEventListener("drop", swallow)
    return () => {
      window.removeEventListener("dragover", swallow)
      window.removeEventListener("drop", swallow)
    }
  }, [])

  return (
    <SettingsProvider>
      <HashRouter>
        <ProjectsProvider>
          <Routes>
            <Route element={<Layout />}>
              <Route index element={<Home />} />
              {/* Terminals are rendered by TerminalHost; the route only selects
                  which one is visible. */}
              <Route path="/projects/:projectId" element={<EmptySessions />} />
              {/* Settings is a per-project screen: it carries the project id so
                  it can show that project's overrides, and renders in the main
                  area with the session sidebar kept beside it. */}
              <Route path="/projects/:projectId/settings" element={<Settings />} />
              {/* The pull-request screen: like Settings, a per-project full-screen
                  route over the terminals. Two shapes of it, and the route is
                  the difference — bare is one checkout's own pull request and
                  nothing else (the worktree's card, the footer badge); "all"
                  adds the repository's list beside it (the tab bar's button),
                  with the number naming the row it selected. */}
              <Route path="/projects/:projectId/pulls" element={<Pulls />} />
              <Route path="/projects/:projectId/pulls/all" element={<Pulls list />} />
              <Route path="/projects/:projectId/pulls/all/:number" element={<Pulls list />} />
            </Route>
          </Routes>
          {/* Inside ProjectsProvider + the router: the update flow opens a shell
              session and navigates to it. */}
          <AppUpdateGate />
          {/* Global quick switcher; also needs the provider (sessions/projects)
              and the router (navigation). */}
          <CommandPalette />
          {/* Read-only list of what is bound; beside the palette because both
              are app-wide overlays opened by a shortcut. */}
          <ShortcutsOverlay />
        </ProjectsProvider>
      </HashRouter>
      {/* Holds its prompt until a provider has been chosen, so a first launch
          asks which harnesses you use before offering to install into them. */}
      <AgentPluginGate />
      <PatchNotesGate />
      {/* A toast, so it shares no surface with the gates around it: the launch
          after a crash can be the launch that also has a release to announce. */}
      <UncleanExitGate />
      {/* Ahead of the provider gate so that on a machine missing both, choosing
          a harness is still the dialog on top: a session with no agent is a
          worse first launch than one with no branch. */}
      <GitMissingGate />
      {/* Last, so that on the one launch where it can coincide with another gate
          it is the dialog on top: choosing a provider comes before reading about
          a release. */}
      <ProviderSetupGate />
      <Toaster />
    </SettingsProvider>
  )
}

export default App
