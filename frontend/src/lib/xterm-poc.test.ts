import {describe, expect, it} from "vitest"
import {isXtermPocEnabled, XTERM_POC_KEY} from "./xterm-poc"

function storageWith(value: string | null): Pick<Storage, "getItem"> {
  return {getItem: (key: string) => (key === XTERM_POC_KEY ? value : null)}
}

describe("isXtermPocEnabled", () => {
  it("is on only when the flag is exactly '1'", () => {
    expect(isXtermPocEnabled(storageWith("1"))).toBe(true)
    expect(isXtermPocEnabled(storageWith("0"))).toBe(false)
    expect(isXtermPocEnabled(storageWith("true"))).toBe(false)
    expect(isXtermPocEnabled(storageWith(null))).toBe(false)
  })

  it("is off without a storage (node/test environment)", () => {
    expect(isXtermPocEnabled(undefined)).toBe(false)
    expect(isXtermPocEnabled(null)).toBe(false)
  })
})
