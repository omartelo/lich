import React from "react"
import ReactDOM from "react-dom/client"
import App from "./App"
import { installBrowserDefaults } from "@/lib/browser-defaults"
import "./index.css"

// Registered once for the page's life, before React: the shell's own reflexes
// must die before any screen can see them, and there is no state to tear down.
installBrowserDefaults(window)

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
