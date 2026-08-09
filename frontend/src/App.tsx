import { useEffect, useState } from "react"
import { HashRouter, Outlet, Route, Routes } from "react-router-dom"
import { SettingsProvider } from "@/providers/settings"
import { ProjectsProvider } from "@/providers/projects"
import { ProjectTabs } from "@/components/tabs/ProjectTabs"
import { SessionSidebar } from "@/components/sidebar/SessionSidebar"
import { TerminalHost } from "@/components/TerminalHost"
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
import { CommandPalette } from "@/components/CommandPalette"

// Layout is persistent across navigation: the project tabs, session sidebar and
// TerminalHost stay mounted while the Outlet swaps screens (Home, Settings) on
// top of the terminals.
function Layout() {
  const [dock, setDock] = useState<DockTab | null>(null)
  const toggleDock = (tab: DockTab) => setDock((cur) => (cur === tab ? null : tab))
  return (
    <div className="flex h-screen w-screen flex-col bg-background">
      <ProjectTabs />
      <div className="flex flex-1 overflow-hidden">
        <SessionSidebar />
        <main className="flex flex-1 flex-col overflow-hidden">
          {/* relative: RightDock overlays this area when in full screen. */}
          <div className="relative flex flex-1 overflow-hidden">
            <div className="relative flex-1 overflow-hidden">
              <TerminalHost />
              <Outlet />
            </div>
            {dock && <RightDock tab={dock} onTab={setDock} onClose={() => setDock(null)} />}
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
        </ProjectsProvider>
      </HashRouter>
      <AgentPluginGate />
      <PatchNotesGate />
      <Toaster />
    </SettingsProvider>
  )
}

export default App
