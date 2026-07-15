import {describe, expect, it} from "vitest"
import {formatGpuProbe, isSoftwareRenderer, mpxPerSec} from "./gpu-probe"
import type {GpuProbeResult} from "./gpu-probe"

describe("isSoftwareRenderer", () => {
  it("flags CPU rasterizers", () => {
    expect(isSoftwareRenderer("llvmpipe (LLVM 19.1.0, 256 bits)")).toBe(true)
    expect(isSoftwareRenderer("Google SwiftShader")).toBe(true)
    expect(isSoftwareRenderer("softpipe")).toBe(true)
    expect(isSoftwareRenderer("Software Rasterizer")).toBe(true)
  })

  it("passes real GPUs", () => {
    expect(isSoftwareRenderer("AMD Radeon RX 7800 XT (radeonsi)")).toBe(false)
    expect(isSoftwareRenderer("Mesa Intel(R) Xe Graphics (TGL GT2)")).toBe(false)
    expect(isSoftwareRenderer("ANGLE (NVIDIA GeForce RTX 4070)")).toBe(false)
  })
})

describe("mpxPerSec", () => {
  it("converts pixels and elapsed time to Mpx/s", () => {
    expect(mpxPerSec(78_643_200, 1000)).toBeCloseTo(78.6, 1)
    expect(mpxPerSec(1_000_000, 100)).toBeCloseTo(10, 5)
  })

  it("returns 0 for a degenerate elapsed time", () => {
    expect(mpxPerSec(1_000_000, 0)).toBe(0)
  })
})

describe("formatGpuProbe", () => {
  const base: GpuProbeResult = {
    webgl2: true,
    renderer: "llvmpipe (LLVM 19.1.0)",
    vendor: "Mesa",
    software: true,
    webglMpxPerSec: 412.4,
    canvas2dMpxPerSec: 890.1,
  }

  it("names a software rasterizer with both fill rates", () => {
    const message = formatGpuProbe(base)
    expect(message).toContain("llvmpipe")
    expect(message).toContain("SOFTWARE rasterizer")
    expect(message).toContain("webgl=412 Mpx/s")
    expect(message).toContain("canvas2d=890 Mpx/s")
  })

  it("names hardware when the renderer is a real GPU", () => {
    const message = formatGpuProbe({
      ...base,
      renderer: "AMD Radeon RX 7800 XT",
      software: false,
    })
    expect(message).toContain("hardware")
    expect(message).not.toContain("SOFTWARE")
  })

  it("declares the migration dead without WebGL2", () => {
    const message = formatGpuProbe({...base, webgl2: false})
    expect(message).toContain("WebGL2 unavailable")
  })

  it("shows n/a when a bench failed", () => {
    const message = formatGpuProbe({...base, webglMpxPerSec: null, canvas2dMpxPerSec: null})
    expect(message).toContain("webgl=n/a")
    expect(message).toContain("canvas2d=n/a")
  })
})
