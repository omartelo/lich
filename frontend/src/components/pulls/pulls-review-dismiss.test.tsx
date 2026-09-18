// @vitest-environment jsdom
//
// Withdrawing a review from the Conversation tab. Resolving every thread a
// review opened leaves its verdict standing: GitHub only moves it when the
// reviewer submits another one or dismisses this one, so this is the second
// door, and the only one lich has ever had for a review nobody wants any more.
//
// The harness is imported before anything that reaches react-dom, for the reason
// render-budget.test.tsx names.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, it, vi } from "vitest"
import type { PullRequestConversation, PullRequestReview } from "@/lib/api-types"
import { PullsConversation } from "./PullsConversation"

const calls = vi.hoisted(() => ({
  dismissed: [] as Array<{ reviewID: string; message: string }>,
  toasts: [] as string[],
}))

vi.mock("sonner", () => ({
  toast: {
    success: (message: string) => calls.toasts.push(message),
    error: (message: string) => calls.toasts.push(message),
  },
}))

const review = (over: Partial<PullRequestReview> = {}): PullRequestReview => ({
  id: "PRR_kw1",
  author: "omartelo",
  state: "CHANGES_REQUESTED",
  body: "the error path swallows the failure",
  date: "2026-07-30T10:00:00Z",
  ...over,
})

const conversation = (...reviews: PullRequestReview[]): PullRequestConversation => ({
  headRefOid: "abc",
  reviews,
  comments: null,
  threads: null,
})

async function mount(convo: PullRequestConversation) {
  return mountBudget(
    createElement(PullsConversation, {
      pull: { projectId: "p1", number: 7 },
      conversation: convo,
      loading: false,
      actions: { reply: async () => {}, resolve: async () => {} },
      onComment: async () => {},
      onDismiss: async (reviewID: string, message: string) => {
        calls.dismissed.push({ reviewID, message })
      },
    }),
  )
}

/** Every button on screen reading exactly this label. */
function buttons(label: string): HTMLButtonElement[] {
  return [...document.querySelectorAll("button")].filter(
    (el): el is HTMLButtonElement => el.textContent?.trim() === label,
  )
}

beforeEach(() => {
  calls.dismissed.length = 0
  calls.toasts.length = 0
  document.body.innerHTML = ""
})

it("offers to withdraw a verdict that still counts", async () => {
  const budget = await mount(conversation(review()))
  expect(buttons("Dismiss")).toHaveLength(1)
  await budget.unmount()
})

it("offers nothing on a verdict there is nothing to withdraw", async () => {
  // A dismissed review is already gone and a bare comment never counted; GitHub
  // refuses both, so neither gets a button that could only come back refused.
  const budget = await mount(
    conversation(
      review({ id: "PRR_kw2", state: "DISMISSED" }),
      review({ id: "PRR_kw3", state: "COMMENTED", body: "reading it now" }),
    ),
  )
  expect(buttons("Dismiss")).toHaveLength(0)
  await budget.unmount()
})

it("offers nothing on a review the conversation read could not address", async () => {
  // Past the query's cap a review arrives without a node id, and the mutation
  // has nothing to aim at.
  const budget = await mount(conversation(review({ id: "" })))
  expect(buttons("Dismiss")).toHaveLength(0)
  await budget.unmount()
})

it("sends the review's node id with the reason typed for it", async () => {
  const budget = await mount(conversation(review()))

  await budget.act(() => buttons("Dismiss")[0].click())
  const field = document.querySelector("textarea")
  if (field === null) {
    throw new Error("the reason box never opened")
  }

  // GitHub refuses a dismissal with no reason, so the box will not send one.
  expect(buttons("Dismiss review")[0].disabled).toBe(true)

  await budget.act(() => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")?.set as (
      this: HTMLTextAreaElement,
      value: string,
    ) => void
    setter.call(field, "fixed in 4a1c2f0")
    field.dispatchEvent(new Event("input", { bubbles: true }))
  })
  await budget.act(() => buttons("Dismiss review")[0].click())

  expect(calls.dismissed).toEqual([{ reviewID: "PRR_kw1", message: "fixed in 4a1c2f0" }])
  expect(calls.toasts).toEqual([])
  await budget.unmount()
})

it("keeps what was typed when GitHub refuses the dismissal", async () => {
  const budget = await mountBudget(
    createElement(PullsConversation, {
      pull: { projectId: "p1", number: 7 },
      conversation: conversation(review()),
      loading: false,
      actions: { reply: async () => {}, resolve: async () => {} },
      onComment: async () => {},
      onDismiss: async () => {
        throw new Error("Resource not accessible")
      },
    }),
  )

  await budget.act(() => buttons("Dismiss")[0].click())
  const field = document.querySelector("textarea")
  if (field === null) {
    throw new Error("the reason box never opened")
  }
  await budget.act(() => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")?.set as (
      this: HTMLTextAreaElement,
      value: string,
    ) => void
    setter.call(field, "no longer applies")
    field.dispatchEvent(new Event("input", { bubbles: true }))
  })
  await budget.act(() => buttons("Dismiss review")[0].click())

  expect(calls.toasts).toEqual(["Dismiss failed: Resource not accessible"])
  // The box stays open with the text in it: a refusal is not a reason to make
  // somebody type the sentence again.
  expect(document.querySelector("textarea")?.value).toBe("no longer applies")
  await budget.unmount()
})
