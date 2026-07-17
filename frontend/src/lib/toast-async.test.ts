import {describe, expect, it, vi, beforeEach} from "vitest"
import {runWithToast} from "./toast-async"

// Sonner is the framework boundary here; stub it so the test asserts the
// control flow (action ran, success/error branch, return value) without a DOM.
const loading = vi.fn(() => "toast-id")
const success = vi.fn()
const error = vi.fn()
vi.mock("sonner", () => ({
  toast: {
    loading: (...args: unknown[]) => loading(...args),
    success: (...args: unknown[]) => success(...args),
    error: (...args: unknown[]) => error(...args),
  },
}))

beforeEach(() => {
  loading.mockClear()
  success.mockClear()
  error.mockClear()
})

describe("runWithToast", () => {
  it("runs the action and resolves the toast to success", async () => {
    const action = vi.fn(async () => undefined)
    const ok = await runWithToast("loading…", action, "done", "failed")

    expect(ok).toBe(true)
    expect(action).toHaveBeenCalledOnce()
    expect(success).toHaveBeenCalledWith("done", {id: "toast-id"})
    expect(error).not.toHaveBeenCalled()
  })

  it("resolves the toast to an error with the message on failure", async () => {
    const action = vi.fn(async () => {
      throw new Error("boom")
    })
    const ok = await runWithToast("loading…", action, "done", "failed")

    expect(ok).toBe(false)
    expect(error).toHaveBeenCalledWith("failed: boom", {id: "toast-id"})
    expect(success).not.toHaveBeenCalled()
  })
})
