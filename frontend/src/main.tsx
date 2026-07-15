import React from 'react'
import ReactDOM from 'react-dom/client'
import { toast } from 'sonner'
import App from './App'
import { installGpuProbe } from './lib/gpu-probe'
import './index.css'

// Dev-only GPU probe (hardware WebGL2 or llvmpipe?); the delayed auto-run
// reports through a toast, so the verdict shows without the inspector.
installGpuProbe((message) => toast(message, { duration: 15000 }))

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
