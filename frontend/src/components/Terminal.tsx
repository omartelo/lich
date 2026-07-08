import { useEffect, useRef } from "react"
import { init, Terminal as Ghostty, FitAddon } from "ghostty-web"
import { Events } from "@wailsio/runtime"
import { Service } from "../../bindings/github.com/skipodotdev/skipo/internals/terminal"

// Event names mirror the backend (internals/terminal).
const DATA_EVENT = "terminal:data"
const EXIT_EVENT = "terminal:exit"

// decodeBase64 turns the base64 PTY payload back into bytes. The backend encodes
// output so multi-byte UTF-8 sequences survive the JSON event bridge intact.
function decodeBase64(data: string): Uint8Array {
  const binary = atob(data)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes
}

export function Terminal() {
  const containerRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }

    let term: Ghostty | null = null
    let fit: FitAddon | null = null
    let disposed = false
    const cleanups: Array<() => void> = []

    const onWindowResize = () => fit?.fit()

    void (async () => {
      await init()
      if (disposed) {
        return
      }

      term = new Ghostty({
        fontSize: 14,
        fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace",
        cursorBlink: true,
        scrollback: 5000,
        theme: { background: "#06070f", foreground: "#e5e7eb" },
      })
      fit = new FitAddon()
      term.loadAddon(fit)
      term.open(container)
      fit.fit()

      // Forward keyboard input and resize events to the PTY.
      const dataInput = term.onData((data) => Service.Write(data))
      const resizeInput = term.onResize(({ cols, rows }) => Service.Resize(cols, rows))
      cleanups.push(() => dataInput.dispose(), () => resizeInput.dispose())

      // Stream PTY output into the terminal.
      const offData = Events.On(DATA_EVENT, (event) => {
        term?.write(decodeBase64(event.data as string))
      })
      const offExit = Events.On(EXIT_EVENT, () => {
        term?.write("\r\n[process exited]\r\n")
      })
      cleanups.push(offData, offExit)

      window.addEventListener("resize", onWindowResize)
      cleanups.push(() => window.removeEventListener("resize", onWindowResize))

      await Service.Start(term.cols, term.rows)
      term.focus()
    })()

    return () => {
      disposed = true
      for (const cleanup of cleanups) {
        cleanup()
      }
      void Service.Close()
      term?.dispose()
    }
  }, [])

  return <div ref={containerRef} className="h-full w-full" />
}
