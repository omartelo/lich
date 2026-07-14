import { useState } from "react"
import { HashRouter, Outlet, Route, Routes } from "react-router-dom"
import { SettingsProvider } from "@/lib/settings"
import { ProjectsProvider } from "@/lib/projects"
import { ProjectTabs } from "@/components/tabs/ProjectTabs"
import { SessionSidebar } from "@/components/sidebar/SessionSidebar"
import { TerminalHost } from "@/components/TerminalHost"
import { DiffPanel } from "@/components/diff/DiffPanel"
import { FooterBar } from "@/components/FooterBar"
import { Home } from "@/components/Home"
import { Settings } from "@/components/settings/Settings"
import { Toaster } from "@/components/ui/sonner"
import { ClaudePluginGate } from "@/components/ClaudePluginGate"

// Layout is persistent across navigation: the project tabs, session sidebar and
// TerminalHost stay mounted while the Outlet swaps screens (Home, Settings) on
// top of the terminals.
function Layout() {
  const [diffOpen, setDiffOpen] = useState(false)
  return (
    <div className="flex h-screen w-screen flex-col bg-background">
      <ProjectTabs />
      <div className="flex flex-1 overflow-hidden">
        <SessionSidebar />
        <main className="flex flex-1 flex-col overflow-hidden">
          {/* relative: DiffPanel overlays this area when in full screen. */}
          <div className="relative flex flex-1 overflow-hidden">
            <div className="relative flex-1 overflow-hidden">
              <TerminalHost />
              <Outlet />
            </div>
            {diffOpen && <DiffPanel onClose={() => setDiffOpen(false)} />}
          </div>
          <FooterBar
            diffOpen={diffOpen}
            onToggleDiff={() => setDiffOpen((v) => !v)}
          />
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
              <Route path="/settings" element={<Settings />} />
            </Route>
          </Routes>
        </ProjectsProvider>
      </HashRouter>
      <ClaudePluginGate />
      <Toaster />
    </SettingsProvider>
  )
}

export default App
