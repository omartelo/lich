// Dev-only probe answering one question: does this WebKitGTK webview have
// hardware WebGL2, or would an xterm.js/WebGL migration land on a software
// rasterizer (llvmpipe/SwiftShader) and be slower than the patched canvas-2d
// pipeline we already have? Two signals:
//
// - the WebGL2 RENDERER/VENDOR strings (llvmpipe-class names mean software).
//   Caveat: WebKit masks both as "Apple GPU (Apple Inc.)" for fingerprinting
//   protection — WebKitGTK on Linux included — so the strings usually carry
//   no identity at all and the verdict leans on the numbers;
// - a fill-rate microbench, WebGL fullscreen-quad draws vs canvas-2d
//   fillRect, in Mpx/s. Hardware GPUs land orders of magnitude above
//   llvmpipe (measured: ~39 Gpx/s on an i7-13th-gen laptop's webview vs
//   ~1-2 Gpx/s llvmpipe-class), so the numbers decide.
//
// Auto-runs once in dev a few seconds after boot (console.info + optional
// report callback for a toast); always exposed as window.__gpuProbe for
// production builds with the inspector enabled. The bench blocks the main
// thread for its duration (~0.5s on llvmpipe) — acceptable for a one-shot
// dev diagnostic, which is why it runs delayed, after the UI is up.

export interface GpuProbeResult {
  webgl2: boolean
  renderer: string
  vendor: string
  software: boolean
  webglMpxPerSec: number | null
  canvas2dMpxPerSec: number | null
}

const BENCH_SIZE = 512
const WEBGL_DRAWS = 300
const CANVAS2D_DRAWS = 300
const AUTORUN_DELAY_MS = 2500
const SOFTWARE_RENDERERS = /llvmpipe|swiftshader|softpipe|swrast|software/i

/** True when a WebGL renderer string names a CPU rasterizer. */
export function isSoftwareRenderer(renderer: string): boolean {
  return SOFTWARE_RENDERERS.test(renderer)
}

/** Fill throughput in Mpx/s from pixels painted and elapsed time. */
export function mpxPerSec(pixels: number, elapsedMs: number): number {
  if (elapsedMs <= 0) {
    return 0
  }
  return pixels / 1e6 / (elapsedMs / 1000)
}

/** One-line human verdict for console/toast. */
export function formatGpuProbe(result: GpuProbeResult): string {
  if (!result.webgl2) {
    return "[gpu-probe] WebGL2 unavailable — xterm.js/WebGL migration is dead (fallback would be the slow DOM renderer)"
  }
  const kind = result.software ? "SOFTWARE rasterizer" : "hardware"
  const webgl = result.webglMpxPerSec != null ? `${Math.round(result.webglMpxPerSec)} Mpx/s` : "n/a"
  const canvas2d =
    result.canvas2dMpxPerSec != null ? `${Math.round(result.canvas2dMpxPerSec)} Mpx/s` : "n/a"
  return (
    `[gpu-probe] ${result.renderer} (${result.vendor}) — ${kind}; ` +
    `fill webgl=${webgl} canvas2d=${canvas2d}`
  )
}

// Fullscreen-quad draws through a real (trivial) fragment shader, so every
// pixel is rasterized — gl.clear would hit the driver's fast-clear path and
// measure memset bandwidth instead of fill rate. A per-draw color uniform
// keeps the driver from coalescing the identical draws; finish() fences the
// GPU before timing stops.
function benchWebglFill(gl: WebGL2RenderingContext): number | null {
  try {
    const program = gl.createProgram()
    const vs = gl.createShader(gl.VERTEX_SHADER)
    const fs = gl.createShader(gl.FRAGMENT_SHADER)
    if (!program || !vs || !fs) {
      return null
    }
    gl.shaderSource(vs, "attribute vec2 p; void main() { gl_Position = vec4(p, 0.0, 1.0); }")
    gl.shaderSource(
      fs,
      "precision mediump float; uniform vec4 c; void main() { gl_FragColor = c; }",
    )
    gl.compileShader(vs)
    gl.compileShader(fs)
    gl.attachShader(program, vs)
    gl.attachShader(program, fs)
    gl.linkProgram(program)
    if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
      return null
    }
    gl.useProgram(program)

    const quad = new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1])
    gl.bindBuffer(gl.ARRAY_BUFFER, gl.createBuffer())
    gl.bufferData(gl.ARRAY_BUFFER, quad, gl.STATIC_DRAW)
    const loc = gl.getAttribLocation(program, "p")
    gl.enableVertexAttribArray(loc)
    gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0)
    const color = gl.getUniformLocation(program, "c")

    gl.uniform4f(color, 0, 0, 0, 1)
    gl.drawArrays(gl.TRIANGLES, 0, 6) // warmup: compile/upload out of the timing
    gl.finish()

    const t0 = performance.now()
    for (let i = 0; i < WEBGL_DRAWS; i++) {
      gl.uniform4f(color, (i % 255) / 255, ((i * 7) % 255) / 255, ((i * 13) % 255) / 255, 1)
      gl.drawArrays(gl.TRIANGLES, 0, 6)
    }
    gl.finish()
    return mpxPerSec(WEBGL_DRAWS * BENCH_SIZE * BENCH_SIZE, performance.now() - t0)
  } catch {
    return null
  }
}

function benchCanvas2dFill(ctx: CanvasRenderingContext2D): number | null {
  try {
    const t0 = performance.now()
    for (let i = 0; i < CANVAS2D_DRAWS; i++) {
      ctx.fillStyle = `rgb(${i % 255},${(i * 7) % 255},${(i * 13) % 255})`
      ctx.fillRect(0, 0, BENCH_SIZE, BENCH_SIZE)
    }
    // Readback forces WebKit to flush the queued paints before timing stops.
    ctx.getImageData(0, 0, 1, 1)
    return mpxPerSec(CANVAS2D_DRAWS * BENCH_SIZE * BENCH_SIZE, performance.now() - t0)
  } catch {
    return null
  }
}

/** Collects renderer identity and fill rates. Safe to call repeatedly. */
export function runGpuProbe(): GpuProbeResult {
  const canvas = document.createElement("canvas")
  canvas.width = BENCH_SIZE
  canvas.height = BENCH_SIZE
  const gl = canvas.getContext("webgl2")
  if (!gl) {
    return {
      webgl2: false,
      renderer: "",
      vendor: "",
      software: false,
      webglMpxPerSec: null,
      canvas2dMpxPerSec: null,
    }
  }

  // UNMASKED_* names the real driver where the extension exists; modern
  // WebKit returns the true string from plain RENDERER/VENDOR already.
  const debugInfo = gl.getExtension("WEBGL_debug_renderer_info") as {
    UNMASKED_RENDERER_WEBGL: number
    UNMASKED_VENDOR_WEBGL: number
  } | null
  const renderer = String(
    (debugInfo && gl.getParameter(debugInfo.UNMASKED_RENDERER_WEBGL)) ??
      gl.getParameter(gl.RENDERER),
  )
  const vendor = String(
    (debugInfo && gl.getParameter(debugInfo.UNMASKED_VENDOR_WEBGL)) ?? gl.getParameter(gl.VENDOR),
  )

  const webglMpxPerSec = benchWebglFill(gl)

  const canvas2d = document.createElement("canvas")
  canvas2d.width = BENCH_SIZE
  canvas2d.height = BENCH_SIZE
  const ctx = canvas2d.getContext("2d")
  const canvas2dMpxPerSec = ctx ? benchCanvas2dFill(ctx) : null

  return {
    webgl2: true,
    renderer,
    vendor,
    software: isSoftwareRenderer(renderer),
    webglMpxPerSec,
    canvas2dMpxPerSec,
  }
}

/**
 * Exposes window.__gpuProbe (all builds) and, in dev, auto-runs the probe
 * shortly after boot, reporting through console.info and the optional
 * callback (e.g. a toast, so the verdict is visible without the inspector).
 */
export function installGpuProbe(report?: (message: string) => void): void {
  ;(window as unknown as { __gpuProbe?: () => GpuProbeResult }).__gpuProbe = runGpuProbe
  if (!import.meta.env.DEV) {
    return
  }
  window.setTimeout(() => {
    const message = formatGpuProbe(runGpuProbe())
    // eslint-disable-next-line no-console
    console.info(message)
    report?.(message)
  }, AUTORUN_DELAY_MS)
}
