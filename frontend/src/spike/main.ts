// Entry for the Chromium spike window (spike.html): one xterm.js + WebGL
// terminal on the spike server's WebSocket. No Wails, no React, no app code —
// the point is to measure Chromium's compositor with the same terminal stack
// the xterm POC uses in the WebKitGTK build. Disposable with cmd/spike.

import { Terminal } from "@xterm/xterm"
import { WebglAddon } from "@xterm/addon-webgl"
import { FitAddon } from "@xterm/addon-fit"
import { startFrameStats } from "./stats"
import "@xterm/xterm/css/xterm.css"

const token = new URLSearchParams(location.search).get("token") ?? ""
const container = document.getElementById("term")
const statsElem = document.getElementById("stats")
if (!container) {
  throw new Error("spike.html is missing #term")
}

const term = new Terminal({
  fontSize: 14,
  fontFamily: "monospace",
  cursorBlink: true,
  scrollback: 5000,
  allowProposedApi: true,
  theme: { background: "#06070f", foreground: "#e5e7eb" },
})
const fit = new FitAddon()
term.loadAddon(fit)
term.open(container)

const webgl = new WebglAddon()
webgl.onContextLoss(() => {
  console.warn("[spike] WebGL context lost, DOM renderer from here on")
  webgl.dispose()
})
term.loadAddon(webgl)
fit.fit()

const ws = new WebSocket(`ws://${location.host}/ws?token=${token}`)
ws.binaryType = "arraybuffer"

function sendControl(msg: object): void {
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(msg))
  }
}

ws.onopen = () => {
  sendControl({ t: "rs", c: term.cols, r: term.rows })
  term.focus()
}
ws.onmessage = (event) => {
  term.write(new Uint8Array(event.data as ArrayBuffer))
}
ws.onclose = () => {
  term.write("\r\n[spike] connection closed\r\n")
}

term.onData((data) => sendControl({ t: "in", d: data }))
term.onResize(({ cols, rows }) => sendControl({ t: "rs", c: cols, r: rows }))

let refitTimer = 0
window.addEventListener("resize", () => {
  window.clearTimeout(refitTimer)
  refitTimer = window.setTimeout(() => fit.fit(), 100)
})

startFrameStats((line) => {
  if (statsElem) {
    statsElem.textContent = line
  }
})
