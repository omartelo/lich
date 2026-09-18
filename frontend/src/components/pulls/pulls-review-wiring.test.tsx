// @vitest-environment jsdom
//
// What the header does with the review chip and what a dismissal costs in reads.
// The chip's own readings and the dismiss box live in their sibling suites; this
// is the seam between them, which neither can see: the chip becoming a way into
// the Conversation tab, and the verdict being re-read from the detail after a
// dismissal, not only from the conversation the timeline came from.
//
// The harness is imported before anything that reaches react-dom, for the reason
// render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, it, vi } from "vitest"
import type {
  PullRequestConversation,
  PullRequestDetail,
  PullRequestReview,
  ReviewThread,
} from "@/lib/api-types"
import { PullRequestView, type SessionAction } from "./PullRequestView"

const calls = vi.hoisted(() => ({
  dismissed: [] as Array<{ path: string; reviewID: string; message: string }>,
  toasts: [] as string[],
}))

vi.mock("sonner", () => ({
  toast: {
    success: (message: string) => calls.toasts.push(message),
    error: (message: string) => calls.toasts.push(message),
  },
}))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    DismissReview: (path: string, reviewID: string, message: string) => {
      calls.dismissed.push({ path, reviewID, message })
      return Promise.resolve(null)
    },
    BranchRules: () => Promise.resolve(null),
  },
  System: { OpenExternal: () => Promise.resolve(null) },
}))

// The tab is a habit read out of storage; this suite needs to know where it
// starts, so it starts where a fresh install does.
vi.mock("@/lib/pulls/pulls-prefs", () => ({
  readPullsTab: () => "overview",
  writePullsTab: () => {},
}))

// The tabs this is not about, stubbed for what they drag in: the diff is
// CodeMirror and the conflict list is another remote read.
vi.mock("./PullsFiles", () => ({ PullsFiles: () => null }))
vi.mock("./PullsConflicts", () => ({ PullsConflicts: () => null }))
vi.mock("./PullsChecks", () => ({ PullsChecks: () => null }))
vi.mock("./PullsCommits", () => ({ PullsCommits: () => null }))
vi.mock("./PullsOverview", () => ({ PullsOverview: () => null }))

const DETAIL: PullRequestDetail = {
  number: 7,
  url: "https://example.invalid/pull/7",
  title: "A pull request",
  body: "",
  author: "someone",
  authorLogin: "someone",
  state: "OPEN",
  isDraft: false,
  mergeable: "MERGEABLE",
  mergeStateStatus: "CLEAN",
  reviewDecision: "CHANGES_REQUESTED",
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

const REVIEW: PullRequestReview = {
  id: "PRR_kw1",
  author: "omartelo",
  state: "CHANGES_REQUESTED",
  body: "the error path swallows the failure",
  date: "2026-07-30T10:00:00Z",
}

const THREAD: ReviewThread = {
  id: "PRRT_kw1",
  path: "internal/project/pr.go",
  line: 61,
  startLine: 0,
  side: "RIGHT",
  isResolved: true,
  isOutdated: false,
  comments: [
    { id: 1, author: "omartelo", body: "here", date: "2026-07-30T10:01:00Z", diffHunk: "@@" },
  ],
}

const SESSION: SessionAction = {
  label: "Open in Session",
  blocked: null,
  busy: false,
  run: () => {},
}

const refreshed = { detail: 0, conversation: 0 }

async function mount(conversation: PullRequestConversation | null, detail = DETAIL) {
  return mountBudget(
    createElement(PullRequestView, {
      path: "/repo",
      projectId: "p1",
      head: "abc123",
      detail,
      session: SESSION,
      onRefresh: () => {
        refreshed.detail++
      },
      onMerged: () => {},
      onInject: () => true,
      onHandOff: async () => {},
      conversation,
      conversationLoading: false,
      onConversationRefresh: () => {
        refreshed.conversation++
      },
    }),
  )
}

const text = () => document.body.textContent ?? ""

function button(label: string): HTMLButtonElement {
  const found = [...document.querySelectorAll("button")].find(
    (el) => el.textContent?.replace(/\s+/g, " ").trim() === label,
  )
  if (!found) {
    throw new Error(`no "${label}" button on screen: ${text()}`)
  }
  return found as HTMLButtonElement
}

function type(field: HTMLTextAreaElement, value: string): void {
  const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")?.set as (
    this: HTMLTextAreaElement,
    next: string,
  ) => void
  setter.call(field, value)
  field.dispatchEvent(new Event("input", { bubbles: true }))
}

beforeEach(() => {
  calls.dismissed.length = 0
  calls.toasts.length = 0
  refreshed.detail = 0
  refreshed.conversation = 0
  document.body.innerHTML = ""
})

it("makes the chip the way into the threads it counts", async () => {
  const budget = await mount({
    headRefOid: "abc",
    reviews: [REVIEW],
    comments: null,
    threads: [THREAD],
  })

  // Overview is where it opened, so the review below is not on screen yet.
  expect(text()).not.toContain("requested changes")

  await budget.act(() => button("Changes requested · all threads resolved").click())
  expect(text()).toContain("requested changes")
  await budget.unmount()
})

it("leaves the chip inert when it counts nothing", async () => {
  // No threads: the chip has no count behind it, so there is nothing to lead to
  // and it stays a readout, the way every other verdict does.
  const budget = await mount({
    headRefOid: "abc",
    reviews: [REVIEW],
    comments: null,
    threads: null,
  })
  expect(() => button("Changes requested")).toThrow()
  await budget.unmount()
})

it("re-reads the verdict as well as the conversation after a dismissal", async () => {
  const budget = await mount({
    headRefOid: "abc",
    reviews: [REVIEW],
    comments: null,
    threads: [THREAD],
  })

  await budget.act(() => button("Changes requested · all threads resolved").click())
  await budget.act(() => button("Dismiss").click())

  const field = document.querySelector("textarea")
  if (field === null) {
    throw new Error("the reason box never opened")
  }
  await budget.act(() => type(field, "answered in full"))
  await budget.act(() => button("Dismiss review").click())

  expect(calls.dismissed).toEqual([
    { path: "/repo", reviewID: "PRR_kw1", message: "answered in full" },
  ])
  // The verdict lives on the detail, the threads on the conversation. Reading
  // only the second leaves the header showing the review just withdrawn.
  expect(refreshed).toEqual({ detail: 1, conversation: 1 })
  expect(calls.toasts).toEqual([])
  await budget.unmount()
})
