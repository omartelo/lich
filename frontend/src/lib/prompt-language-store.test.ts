import { beforeEach, describe, expect, it, vi } from "vitest"
import {
  promptLanguageStore,
  resetPromptLanguageStore,
  setPromptLanguage,
} from "./prompt-language-store"

const getSetting = vi.fn()
const setSetting = vi.fn()
vi.mock("@/lib/rpc", () => ({
  Store: {
    GetSetting: (key: string, projectID: string) => getSetting(key, projectID),
    SetSetting: (key: string, projectID: string, value: string) =>
      setSetting(key, projectID, value),
  },
}))

const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

beforeEach(() => {
  resetPromptLanguageStore()
  getSetting.mockReset().mockResolvedValue("")
  setSetting.mockReset().mockResolvedValue(null)
})

describe("promptLanguageStore", () => {
  it("reads English before and without a stored value", async () => {
    expect(promptLanguageStore.get()).toBe("en")
    promptLanguageStore.subscribe(() => {})
    await flush()
    expect(getSetting).toHaveBeenCalledWith("prompt.language", "")
    expect(promptLanguageStore.get()).toBe("en")
  })

  it("loads the stored language once, on the first subscriber", async () => {
    getSetting.mockResolvedValue("pt-BR")
    const listener = vi.fn()
    promptLanguageStore.subscribe(listener)
    promptLanguageStore.subscribe(() => {})
    await flush()
    expect(getSetting).toHaveBeenCalledTimes(1)
    expect(promptLanguageStore.get()).toBe("pt-BR")
    expect(listener).toHaveBeenCalledTimes(1)
  })

  it("keeps English when the read fails", async () => {
    getSetting.mockRejectedValue(new Error("down"))
    promptLanguageStore.subscribe(() => {})
    await flush()
    expect(promptLanguageStore.get()).toBe("en")
  })

  it("writes a choice through to the global setting", async () => {
    await setPromptLanguage("pt-BR")
    expect(setSetting).toHaveBeenCalledWith("prompt.language", "", "pt-BR")
    expect(promptLanguageStore.get()).toBe("pt-BR")
  })

  it("puts the old language back when the write is refused", async () => {
    setSetting.mockRejectedValue(new Error("locked"))
    await expect(setPromptLanguage("pt-BR")).rejects.toThrow("locked")
    expect(promptLanguageStore.get()).toBe("en")
  })

  it("lets a choice made during the load win over the loaded value", async () => {
    getSetting.mockResolvedValue("en")
    promptLanguageStore.subscribe(() => {})
    await setPromptLanguage("pt-BR")
    await flush()
    expect(promptLanguageStore.get()).toBe("pt-BR")
  })
})
