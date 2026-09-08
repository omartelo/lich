// @vitest-environment jsdom
//
// What the pull request screen's session button *does*, as opposed to what it
// says. The label is drawn from the filed checkout list, which can be a round
// trip behind a worktree removed from a terminal; the click re-reads before it
// picks a path, so the two can disagree and the action still lands.
//
// The harness is imported before anything that reaches react-dom, for the
// reason render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, it, vi } from "vitest"
import { clearRemoteCache } from "@/lib/remote-cache"
import type { PullRequestDetail, Worktree } from "@/lib/api-types"
import type { SessionAction } from "./PullRequestView"
import { Pulls } from "./Pulls"

const PROJECT_ID = "p1"
const PROJECT_PATH = "/repo"
const WORKTREE: Worktree = { name: "feature", path: "/checkouts/feature" }
const LIVE_SESSION = "s-live"

const DETAIL: PullRequestDetail = {
  number: 7,
  url: "https://example.invalid/pull/7",
  title: "A pull request",
  body: "",
  author: "someone",
  state: "OPEN",
  isDraft: false,
  mergeable: "MERGEABLE",
  mergeStateStatus: "CLEAN",
  reviewDecision: "",
  reviewers: null,
  baseRefName: "main",
  headRefName: "feature",
  changedFiles: 1,
  isCrossRepository: false,
  maintainerCanModify: false,
  checks: { passed: 0, failed: 0, pending: 0, total: 0 },
  checkRuns: null,
  commits: null,
}

const calls = vi.hoisted(() => ({
  /** What the next ListCheckouts answers. Swapped per test, and mid-test. */
  list: () => Promise.resolve([] as Worktree[]),
  created: [] as number[],
  reopened: [] as string[],
  activated: [] as string[],
  removed: [] as string[],
  /** Every toast raised, newest last, with the action a cleanup offer carries. */
  toasts: [] as Array<{ message: string; action?: () => void }>,
}))

vi.mock("sonner", () => ({
  toast: {
    success: (message: string, options?: { action?: { onClick: () => void } }) => {
      calls.toasts.push({ message, action: options?.action?.onClick })
    },
    error: (message: string) => {
      calls.toasts.push({ message })
    },
  },
}))

/** A read that never lands, so what is on screen came from the filed answer. */
const pending = () => new Promise<Worktree[]>(() => {})

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    ListCheckouts: () => calls.list(),
    PullRequestDetail: () => Promise.resolve(DETAIL),
    PullRequestConversation: () => Promise.resolve(null),
    ListPullRequests: () => Promise.resolve([]),
    CreateWorktreeFromPR: (_path: string, _projectId: string, number: number) => {
      calls.created.push(number)
      return Promise.resolve(WORKTREE)
    },
    WorktreeAdopted: () => Promise.resolve(false),
    WorktreeDirty: () => Promise.resolve(false),
    RemoveWorktree: (_projectPath: string, wtPath: string) => {
      calls.removed.push(wtPath)
      return Promise.resolve(null)
    },
  },
  Store: { PurgeWorktreeSessions: () => Promise.resolve(null) },
  Terminal: { Write: () => Promise.resolve(null) },
}))

vi.mock("react-router-dom", () => ({
  useNavigate: () => () => {},
  useParams: () => ({ projectId: PROJECT_ID }),
}))

// A session already living in the worktree, which is what makes the button read
// "Go to session" rather than offering to open one.
vi.mock("@/providers/projects", () => ({
  useProjects: () => ({
    projects: [{ id: PROJECT_ID, path: PROJECT_PATH }],
    sessions: {
      [PROJECT_ID]: {
        sessions: [{ id: LIVE_SESSION, path: WORKTREE.path }],
        activeId: LIVE_SESSION,
        nextSeq: 2,
      },
    },
    discardSession: () => {},
    newSession: () => "s-new",
    newWorktreeSession: () => "s-worktree",
    reopenWorktreeSession: (_projectId: string, wt: Worktree) => {
      calls.reopened.push(wt.path)
      return Promise.resolve("s-reopened")
    },
    activateSession: (_projectId: string, id: string) => {
      calls.activated.push(id)
    },
  }),
}))

// gh is answered at once: the screen draws nothing at all until it has.
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

// The pull request itself is not what this is about — only what Pulls hands it:
// the session button, with the label Pulls decided and the run Pulls wired, and
// the merge that raises Pulls' own cleanup offer.
vi.mock("./PullRequestView", () => ({
  PullRequestView: ({ session, onMerged }: { session: SessionAction; onMerged: () => void }) => [
    createElement("button", { key: "s", type: "button", onClick: session.run }, session.label),
    createElement("button", { key: "m", type: "button", onClick: onMerged }, "Merge"),
  ],
}))

const text = () => document.body.textContent ?? ""

function click(label: string): void {
  const button = [...document.querySelectorAll("button")].find(
    (element) => element.textContent?.trim() === label,
  )
  if (!button) {
    throw new Error(`no "${label}" button on screen: ${text()}`)
  }
  button.click()
}

const clickSessionButton = (): void => {
  const button = document.querySelector("button")
  if (!button) {
    throw new Error(`no session button on screen: ${text()}`)
  }
  button.click()
}

const lastToast = () => calls.toasts[calls.toasts.length - 1]

beforeEach(() => {
  clearRemoteCache()
  calls.list = () => Promise.resolve([])
  calls.created = []
  calls.reopened = []
  calls.activated = []
  calls.removed = []
  calls.toasts = []
})

// The trap: the filed list still holds a checkout git no longer has, so the
// button offers to go to a session in a directory that is gone. Acting on the
// filed answer hands that path to the session flows; acting on a fresh read
// falls through to creating the checkout, with the same gesture.
it("creates a worktree when the filed checkout is gone by the time it is clicked", async () => {
  calls.list = () => Promise.resolve([WORKTREE])
  const first = await mountBudget(createElement(Pulls))
  await first.act(() => {})
  await first.unmount()

  // This visit's read never lands, so the button on screen is the filed answer
  // and nothing else — the one frame the cache exists for.
  calls.list = pending
  const second = await mountBudget(createElement(Pulls))
  expect(text()).toContain("Go to session")

  // Removed from a terminal in the meantime, answered to the click's own read.
  calls.list = () => Promise.resolve([])
  await second.act(() => clickSessionButton())

  expect(calls.created).toEqual([DETAIL.number])
  expect(calls.activated).toEqual([])
  expect(calls.reopened).toEqual([])
  await second.unmount()
})

// The merge toast carries the same filed answer for ten seconds, which is long
// enough for the checkout to go away under it — from the sidebar, or from a
// terminal. A remove that has already happened is what the offer was asking
// for, so it is said in the toast's own voice rather than reported as a failure.
it("says the worktree was already removed instead of failing the cleanup", async () => {
  calls.list = () => Promise.resolve([WORKTREE])
  const mounted = await mountBudget(createElement(Pulls))
  await mounted.act(() => {})

  await mounted.act(() => click("Merge"))
  const offer = lastToast()
  expect(offer?.action).toBeDefined()

  // Removed under the toast, answered to the offer's own read.
  calls.list = () => Promise.resolve([])
  await mounted.act(() => offer?.action?.())

  expect(calls.removed).toEqual([])
  expect(lastToast()?.message).toBe("feature was already removed")
  await mounted.unmount()
})

// The other half of the same read: a checkout that is still there is reused,
// which is the whole reason the screen asks before it creates one.
it("goes to the session when the checkout is still there", async () => {
  calls.list = () => Promise.resolve([WORKTREE])
  const mounted = await mountBudget(createElement(Pulls))
  await mounted.act(() => {})
  expect(text()).toContain("Go to session")

  await mounted.act(() => clickSessionButton())

  expect(calls.activated).toEqual([LIVE_SESSION])
  expect(calls.created).toEqual([])
  await mounted.unmount()
})
