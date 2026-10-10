// @vitest-environment jsdom
//
// What the empty Pulls screen's "Create with agent" does: the branch with no
// pull request is the active session's own, so the prompt is written there and
// the screen steps aside for it.
//
// The harness is imported before anything that reaches react-dom, for the
// reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, it, vi } from "vitest"
import { clearRemoteCache } from "@/lib/remote-cache"
import { Pulls } from "./Pulls"
import { hoverHint } from "@/test/hint"

const PROJECT_ID = "p1"
const PROJECT_PATH = "/repo"

const calls = vi.hoisted(() => ({
  sessionId: "s1",
  branch: "feature",
  written: [] as Array<{ sessionId: string; text: string }>,
  navigated: [] as string[],
}))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    ListCheckouts: () => Promise.resolve([]),
    PullRequestDetail: () => Promise.resolve(null),
    PullRequestConversation: () => Promise.resolve(null),
    ListPullRequests: () => Promise.resolve([]),
  },
  Store: { PurgeWorktreeSessions: () => Promise.resolve(null) },
  Terminal: { Write: () => Promise.resolve(null) },
}))

vi.mock("@/lib/terminal/write-at-prompt", () => ({
  writeAtPrompt: (sessionId: string, text: string) => {
    calls.written.push({ sessionId, text })
    return Promise.resolve()
  },
}))

vi.mock("react-router-dom", () => ({
  useNavigate: () => (to: string) => {
    calls.navigated.push(to)
  },
  useParams: () => ({ projectId: PROJECT_ID }),
}))

vi.mock("@/providers/projects", () => ({
  useProjects: () => ({
    projects: [{ id: PROJECT_ID, path: PROJECT_PATH }],
    sessions: {},
    discardSession: () => {},
    newSession: () => "s-new",
    newWorktreeSession: () => "s-worktree",
    reopenWorktreeSession: () => Promise.resolve("s-reopened"),
    activateSession: () => {},
  }),
}))

vi.mock("@/lib/use-binary-check", () => ({
  NO_SETTLE: 0,
  useBinaryCheck: () => ({ path: "/usr/bin/gh", status: "ok" }),
}))

vi.mock("@/lib/session/use-active-session", () => ({
  useActiveSession: () => ({
    sessionId: calls.sessionId,
    path: PROJECT_PATH,
    checkout: PROJECT_PATH,
  }),
}))

vi.mock("@/lib/git/use-git-status", () => ({
  useGitStatus: () => ({ branch: calls.branch, head: "abc123" }),
}))

function agentButton(): HTMLButtonElement {
  const button = [...document.querySelectorAll("button")].find((element) =>
    element.textContent?.includes("Create with agent"),
  )
  if (!button) {
    throw new Error(`no "Create with agent" button on screen: ${document.body.textContent}`)
  }
  return button
}

beforeEach(() => {
  clearRemoteCache()
  calls.sessionId = "s1"
  calls.branch = "feature"
  calls.written = []
  calls.navigated = []
})

it("writes the pull request prompt at the active session and goes to it", async () => {
  const mounted = await mountBudget(createElement(Pulls))
  await mounted.act(() => {})

  await mounted.act(() => agentButton().click())

  expect(calls.navigated).toEqual([`/projects/${PROJECT_ID}`])
  expect(calls.written).toHaveLength(1)
  expect(calls.written[0]?.sessionId).toBe("s1")
  expect(calls.written[0]?.text).toContain("Branch feature has no pull request yet")
  await mounted.unmount()
})

it("offers nothing to hand over without a session on the checkout", async () => {
  calls.sessionId = ""
  const mounted = await mountBudget(createElement(Pulls))
  await mounted.act(() => {})

  expect(agentButton().disabled).toBe(true)
  // Contract changed: the reason rides a Hint on the wrapper instead of its title.
  expect(await hoverHint(agentButton().parentElement as Element)).toBe(
    "No session on this checkout to hand it to",
  )
  await mounted.unmount()
})

it("offers nothing to hand over on a detached HEAD", async () => {
  calls.branch = ""
  const mounted = await mountBudget(createElement(Pulls))
  await mounted.act(() => {})

  expect(agentButton().disabled).toBe(true)
  // Contract changed: the reason rides a Hint on the wrapper instead of its title.
  expect(await hoverHint(agentButton().parentElement as Element)).toBe("HEAD is not on a branch")
  await mounted.unmount()
})
