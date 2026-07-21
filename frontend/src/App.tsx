import { useState } from "react"
import { HashRouter, Outlet, Route, Routes } from "react-router-dom"
import { SettingsProvider } from "@/lib/settings"
import { ProjectsProvider } from "@/lib/projects"
import { ProjectTabs } from "@/components/tabs/ProjectTabs"
import { SessionSidebar } from "@/components/sidebar/SessionSidebar"
import { TerminalHost } from "@/components/TerminalHost"
import { RightDock, type DockTab } from "@/components/dock/RightDock"
import { FooterBar } from "@/components/FooterBar"
import { Home } from "@/components/Home"
import { Settings } from "@/components/settings/Settings"
import { Toaster } from "@/components/ui/sonner"
import { ClaudePluginGate } from "@/components/ClaudePluginGate"
import { AppUpdateGate } from "@/components/AppUpdateGate"
import { CommandPalette } from "@/components/CommandPalette"

// Layout is persistent across navigation: the project tabs, session sidebar and
// TerminalHost stay mounted while the Outlet swaps screens (Home, Settings) on
// top of the terminals.
function Layout() {
  const [dock, setDock] = useState<DockTab | null>(null)
  // Clicking a footer toggle opens that tab, or closes the dock if that tab is
  // already showing.
  const toggleDock = (tab: DockTab) =>
    setDock((cur) => (cur === tab ? null : tab))
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
            {dock && (
              <RightDock
                tab={dock}
                onTab={setDock}
                onClose={() => setDock(null)}
              />
            )}
          </div>
          <FooterBar dock={dock} onDock={toggleDock} />
        </main>
      </div>
    </div>
  )
}

function App() {
  return (
    <SettingsProvider>
      <HashRouter>
        <ProjectsProvider>
          <Routes>
            <Route element={<Layout />}>
              <Route index element={<Home />} />
              {/* Terminals are rendered by TerminalHost; the route only selects
                  which one is visible, so this element is empty. */}
              <Route path="/projects/:projectId" element={null} />
              {/* Settings is a per-project screen: it carries the project id so
                  it can show that project's overrides, and renders in the main
                  area with the session sidebar kept beside it. */}
              <Route path="/projects/:projectId/settings" element={<Settings />} />
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
      <ClaudePluginGate />
      <Toaster />
    </SettingsProvider>
  )
}

export default App
