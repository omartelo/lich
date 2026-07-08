import { HashRouter, Navigate, Outlet, Route, Routes } from "react-router-dom"
import { SettingsProvider } from "@/lib/settings"
import { Rail } from "@/components/Rail"
import { TerminalHost } from "@/components/TerminalHost"
import { Settings } from "@/components/Settings"

// Layout is persistent across navigation: the Rail and the TerminalHost stay
// mounted while the Outlet swaps screens on top of the terminals.
function Layout() {
  return (
    <div className="flex h-screen w-screen bg-background">
      <Rail />
      <main className="relative flex-1 overflow-hidden">
        <TerminalHost />
        <Outlet />
      </main>
    </div>
  )
}

function App() {
  return (
    <SettingsProvider>
      <HashRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route index element={<Navigate to="/projects/home" replace />} />
            {/* Terminals are rendered by TerminalHost; the route only selects
                which one is visible, so this element is empty. */}
            <Route path="/projects/:id" element={null} />
            <Route path="/settings" element={<Settings />} />
          </Route>
        </Routes>
      </HashRouter>
    </SettingsProvider>
  )
}

export default App
