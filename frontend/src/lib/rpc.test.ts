import {describe, expect, it} from "vitest"
import {endpointFromLocation} from "./rpc"

describe("endpointFromLocation", () => {
  it("extracts base and token from a Chromium-shell URL", () => {
    expect(endpointFromLocation("http://127.0.0.1:43121/index.html?token=abc123")).toEqual({
      base: "http://127.0.0.1:43121",
      token: "abc123",
    })
  })

  it("returns null without a token (Wails webview URLs)", () => {
    expect(endpointFromLocation("wails://wails.localhost/")).toBeNull()
    expect(endpointFromLocation("http://127.0.0.1:9245/")).toBeNull()
  })

  it("returns null on unparsable input", () => {
    expect(endpointFromLocation("::: not a url")).toBeNull()
  })
})
