import { beforeEach, expect, test, vi } from "vitest"
import { carryInto } from "./carry"

const calls = vi.hoisted(() => ({
  copied: [] as Array<[string, string]>,
  fail: "",
  toasts: [] as Array<[string, string]>,
}))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    CarryUncommitted: (from: string, to: string) => {
      calls.copied.push([from, to])
      return calls.fail ? Promise.reject(new Error(calls.fail)) : Promise.resolve(null)
    },
  },
}))

vi.mock("sonner", () => ({
  toast: {
    error: (message: string) => calls.toasts.push(["error", message]),
    warning: (message: string) => calls.toasts.push(["warning", message]),
  },
}))

const worktree = { name: "option-b", path: "/wt/option-b" }

beforeEach(() => {
  calls.copied = []
  calls.fail = ""
  calls.toasts = []
})

test("a worktree that is not a fork of a working tree copies nothing", async () => {
  await carryInto("", worktree)

  expect(calls.copied).toEqual([])
  expect(calls.toasts).toEqual([])
})

test("the fork's checkout is copied into the new worktree", async () => {
  await carryInto("/wt/icy-glacier", worktree)

  expect(calls.copied).toEqual([["/wt/icy-glacier", "/wt/option-b"]])
  expect(calls.toasts).toEqual([])
})

test("a failed copy is said out loud and never thrown: the session still opens", async () => {
  calls.fail = "patch does not apply"

  await expect(carryInto("/wt/icy-glacier", worktree)).resolves.toBeUndefined()
  expect(calls.toasts[0][0]).toBe("error")
  expect(calls.toasts[0][1]).toContain("patch does not apply")
})

test("a branch that already existed says so instead of copying onto it", async () => {
  await carryInto("/wt/icy-glacier", { ...worktree, reused: true })

  // git would refuse the patch whole — it was read against another commit —
  // and the base the user picked decided nothing either.
  expect(calls.copied).toEqual([])
  expect(calls.toasts[0][0]).toBe("warning")
  expect(calls.toasts[0][1]).toContain("option-b")
})
