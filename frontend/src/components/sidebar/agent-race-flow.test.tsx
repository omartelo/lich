// @vitest-environment jsdom
//
// A race from start to finish through the window's own seams: one worktree and
// one filed session per agent with the task sent to each, and the keep that
// ends it removing every other checkout, forced, and unfiling what is left.
// Everything past those seams (git, the PTY, the database) is the RPC mock.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import type { Session } from "@/lib/session/sessions"
import { rememberConsolidator } from "@/lib/session/agent-race"
import { useAgentRace } from "./useAgentRace"
import { WorktreeCloseDialogs } from "./WorktreeCloseDialogs"
import { type WorktreeClose, useWorktreeClose } from "./useWorktreeClose"

const calls = vi.hoisted(() => ({
  created: [] as string[],
  failOn: "",
  opened: [] as { name: string; kind: string; folder: string }[],
  sent: [] as [string, string][],
  discarded: [] as string[],
  removed: [] as [string, boolean][],
  filed: [] as [string[], string][],
  running: [] as string[],
}))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    CreateWorktree: (_project: string, _id: string, name: string) => {
      if (name === calls.failOn) {
        return Promise.reject(new Error("branch already checked out"))
      }
      calls.created.push(name)
      return Promise.resolve({ name, path: `/wt/${name}` })
    },
    RemoveWorktree: (_project: string, path: string, force: boolean) => {
      calls.removed.push([path, force])
      return Promise.resolve(null)
    },
    WorktreeAdopted: () => Promise.resolve(false),
  },
  Store: { PurgeWorktreeSessions: () => Promise.resolve(null) },
}))

vi.mock("@/providers/projects", () => ({
  useProjects: () => ({
    newWorktreeSession: (
      _projectId: string,
      wt: { name: string },
      _sandbox: string,
      _from: unknown,
      kind: string,
      folder: string,
    ) => {
      calls.opened.push({ name: wt.name, kind, folder })
      return `s-${wt.name}`
    },
    discardSession: (_projectId: string, id: string) => calls.discarded.push(id),
    fileSessions: (_projectId: string, ids: string[], folder: string) =>
      calls.filed.push([ids, folder]),
    closeSession: () => {},
    keepSession: () => {},
  }),
}))

vi.mock("@/lib/git/carry", () => ({ carryInto: () => Promise.resolve() }))
vi.mock("@/lib/terminal/setup-queue", () => ({ queueSetup: () => {} }))
vi.mock("@/lib/terminal/write-at-prompt", () => ({
  sendAtPrompt: (id: string, text: string) => {
    calls.sent.push([id, text])
    return Promise.resolve()
  },
}))
vi.mock("@/lib/session/use-session-status", () => ({
  runningSessions: (ids: string[]) => ids.filter((id) => calls.running.includes(id)),
}))

beforeEach(() => {
  calls.created = []
  calls.failOn = ""
  calls.opened = []
  calls.sent = []
  calls.discarded = []
  calls.removed = []
  calls.filed = []
  calls.running = []
})

const main = { branch: "main", remote: false, carryFrom: "" }

test("a race makes one worktree and one filed session per agent, and sends each the task", async () => {
  const race = useAgentRace("p1", "/repo")

  const started = await race.start("fix-auth", main, "", "fix the redirect", ["claude", "kiro"])

  expect(started).toEqual({
    folder: "fix-auth",
    sessionIds: ["s-fix-auth-claude", "s-fix-auth-kiro"],
  })
  expect(calls.created).toEqual(["fix-auth-claude", "fix-auth-kiro"])
  expect(calls.opened).toEqual([
    { name: "fix-auth-claude", kind: "claude", folder: "fix-auth" },
    { name: "fix-auth-kiro", kind: "kiro", folder: "fix-auth" },
  ])
  expect(calls.sent).toEqual([
    ["s-fix-auth-claude", "fix the redirect"],
    ["s-fix-auth-kiro", "fix the redirect"],
  ])
})

test("a consolidation opens one worktree in the race's folder and sends it the prompt", async () => {
  const race = useAgentRace("p1", "/repo")

  const opened = await race.consolidate(
    "fix-auth",
    "fix-auth-consolidated",
    main,
    "",
    "combine",
    "codex",
  )

  expect(opened).toBe("s-fix-auth-consolidated")
  expect(calls.opened).toEqual([
    { name: "fix-auth-consolidated", kind: "codex", folder: "fix-auth" },
  ])
  expect(calls.sent).toEqual([["s-fix-auth-consolidated", "combine"]])
})

test("a refusal halfway names the agent and keeps the ones already started", async () => {
  calls.failOn = "fix-auth-codex"
  const race = useAgentRace("p1", "/repo")

  await expect(
    race.start("fix-auth", main, "", "fix it", ["claude", "codex", "kiro"]),
  ).rejects.toThrow("Started 1 of 3 agents; fix-auth-codex failed: branch already checked out")
  expect(calls.created).toEqual(["fix-auth-claude"])
  expect(calls.sent).toEqual([["s-fix-auth-claude", "fix it"]])
})

const winner: Session = { id: "w", label: "a-claude", kind: "claude", path: "/wt/a", folder: "a" }
const codex: Session = { id: "c", label: "a-codex", kind: "codex", path: "/wt/b", folder: "a" }
const shell: Session = { id: "t", label: "Terminal", kind: "shell", path: "/wt/b", folder: "a" }
const beside: Session = { id: "x", label: "Session 9", kind: "shell", path: "/wt/a", folder: "a" }

function harness(sessions: Session[], out: { close: WorktreeClose | null }) {
  function Probe() {
    out.close = useWorktreeClose("p1", "/repo", sessions)
    return null
  }
  return createElement(Probe)
}

test("keeping the winner removes every other checkout forced and unfiles what is left", async () => {
  calls.running = ["c"]
  const out: { close: WorktreeClose | null } = { close: null }
  const mounted = await mountBudget(harness([winner, codex, shell, beside], out))

  await mounted.act(() => out.close?.requestKeepWinner(winner))
  expect(out.close?.pendingKeep).toEqual({
    folder: "a",
    winner,
    consolidated: false,
    rivals: [{ path: "/wt/b", sessions: [codex, shell] }],
    running: ["c"],
  })

  await mounted.act(() => out.close?.keepWinner())
  expect(calls.discarded).toEqual(["c", "t"])
  expect(calls.removed).toEqual([["/wt/b", true]])
  expect(calls.filed).toEqual([[["w", "x"], ""]])
  expect(out.close?.pendingKeep).toBeNull()
  await mounted.unmount()
})

test("a session with no rivals asks nothing", async () => {
  const out: { close: WorktreeClose | null } = { close: null }
  const mounted = await mountBudget(harness([winner, beside], out))

  await mounted.act(() => out.close?.requestKeepWinner(winner))

  expect(out.close?.pendingKeep).toBeNull()
  expect(out.close?.rivalsOf(winner)).toEqual([])
  await mounted.unmount()
})

test("cancel drops the pending keep and removes nothing", async () => {
  const out: { close: WorktreeClose | null } = { close: null }
  const mounted = await mountBudget(harness([winner, codex], out))

  await mounted.act(() => out.close?.requestKeepWinner(winner))
  await mounted.act(() => out.close?.cancel())
  await mounted.act(() => out.close?.keepWinner())

  expect(calls.removed).toEqual([])
  await mounted.unmount()
})

test("removing the race takes every worktree in the folder and keeps none", async () => {
  const out: { close: WorktreeClose | null } = { close: null }
  const mounted = await mountBudget(harness([winner, codex, shell], out))

  await mounted.act(() => out.close?.requestRemoveRace("a"))
  expect(out.close?.pendingKeep?.winner).toBeNull()
  expect(out.close?.pendingKeep?.rivals.map((rival) => rival.path)).toEqual(["/wt/a", "/wt/b"])

  await mounted.act(() => out.close?.keepWinner())
  expect(calls.removed).toEqual([
    ["/wt/a", true],
    ["/wt/b", true],
  ])
  expect(calls.filed).toEqual([[[], ""]])
  await mounted.unmount()
})

// Keeping the consolidation is the end of the race, and the confirmation says
// so rather than reading as one agent beating the others.
test("keeping the race's consolidation asks to remove the race", async () => {
  rememberConsolidator("p1", "a", "w")
  const out: { close: WorktreeClose | null } = { close: null }
  const mounted = await mountBudget(harness([winner, codex], out))

  await mounted.act(() => out.close?.requestKeepWinner(winner))
  expect(out.close?.pendingKeep?.consolidated).toBe(true)
  await mounted.unmount()

  const close = out.close
  if (!close) {
    throw new Error("close flow not captured")
  }
  const dialogs = await mountBudget(createElement(WorktreeCloseDialogs, { close }))
  await dialogs.act(async () => {})
  expect(document.body.textContent).toContain("Remove the a race?")
  expect(document.body.textContent).toContain("a-claude is kept.")
  await dialogs.unmount()
})
