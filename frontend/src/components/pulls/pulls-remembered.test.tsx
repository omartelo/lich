// @vitest-environment jsdom
//
// Which pull request the list screen reopens on. The bare list route carries no
// number, so the screen stands in the one it last had selected — but only while
// that one is still in the list: a pull request merged since is not what a
// list of open ones should open on.
//
// The harness is imported before anything that reaches react-dom, for the
// reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, it, vi } from "vitest"
import type { PullRequestSummary } from "@/lib/api-types"
import { clearRemoteCache } from "@/lib/remote-cache"
import { writeLastPull } from "@/lib/pulls/pulls-prefs"
import { Pulls } from "./Pulls"

const PROJECT_ID = "p1"
const PROJECT_PATH = "/repo"
const REMEMBERED = 58

const calls = vi.hoisted(() => ({
  open: [] as PullRequestSummary[],
  detailNumbers: [] as number[],
}))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    ListCheckouts: () => Promise.resolve([]),
    PullRequestDetail: (_path: string, number: number) => {
      calls.detailNumbers.push(number)
      return Promise.resolve(null)
    },
    PullRequestConversation: () => Promise.resolve(null),
    ListPullRequests: () => Promise.resolve(calls.open),
  },
  Store: { PurgeWorktreeSessions: () => Promise.resolve(null) },
  Terminal: { Write: () => Promise.resolve(null) },
}))

vi.mock("react-router-dom", () => ({
  useNavigate: () => () => {},
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
  useActiveSession: () => ({ sessionId: "s1", path: PROJECT_PATH, checkout: PROJECT_PATH }),
}))

vi.mock("@/lib/git/use-git-status", () => ({
  useGitStatus: () => ({ branch: "main", head: "abc123" }),
}))

function pr(number: number): PullRequestSummary {
  return {
    number,
    title: `feat: pull ${number}`,
    author: "omartelo",
    state: "OPEN",
    isDraft: false,
    reviewDecision: "",
    headRefName: `feat/${number}`,
    isCrossRepository: false,
    updatedAt: "2026-07-26T10:00:00Z",
    checks: { passed: 0, failed: 0, pending: 0, total: 0 },
  }
}

beforeEach(() => {
  clearRemoteCache()
  localStorage.clear()
  writeLastPull(PROJECT_ID, REMEMBERED)
  calls.detailNumbers = []
})

it("does not reopen on a remembered pull request that is no longer open", async () => {
  calls.open = []
  const mounted = await mountBudget(createElement(Pulls, { list: true }))
  await mounted.act(() => {})

  expect(calls.detailNumbers).not.toContain(REMEMBERED)
  expect(document.body.textContent).toContain("No open pull requests")
  await mounted.unmount()
})

it("reopens on the remembered pull request while it is still open", async () => {
  calls.open = [pr(REMEMBERED), pr(12)]
  const mounted = await mountBudget(createElement(Pulls, { list: true }))
  await mounted.act(() => {})

  expect(calls.detailNumbers).toEqual([REMEMBERED])
  await mounted.unmount()
})
