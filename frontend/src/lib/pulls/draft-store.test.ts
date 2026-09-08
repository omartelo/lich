// @vitest-environment jsdom
//
// jsdom for its localStorage alone, and nothing here renders. The store reads
// storage as it loads — that is what a draft surviving a reload *is* — so a
// stub installed by the test body would arrive too late, and enumerating the
// keys is half of what the sweep does.
import { beforeEach, describe, expect, it, vi } from "vitest"
import {
  DRAFT_MAX_AGE_MS,
  type DraftScope,
  draftKey,
  draftStore,
  parseDraft,
  setDraft,
  sweepDrafts,
} from "./draft-store"

const PROJECT = "a1b2c3d4e5f6"
const OTHER_PROJECT = "0f0f0f0f0f0f"
const PR: DraftScope = { projectId: PROJECT, number: 389 }
const OTHER: DraftScope = { projectId: PROJECT, number: 12 }
const THREAD = "PRRT_kwDOabc"
// The highest number a swept list carries: nothing above it can be judged
// closed, so every sweep that means to collect something says it out loud.
const NEWEST = 400

// A page load: the module reads storage once as it evaluates, so a fresh copy
// of it is the only honest way to ask what the next launch would restore.
async function reload() {
  vi.resetModules()
  return await import("./draft-store")
}

beforeEach(() => {
  localStorage.clear()
  for (const key of [
    draftKey(PR, "body"),
    draftKey(PR, "comment"),
    draftKey(PR, "reply", THREAD),
    draftKey(OTHER, "comment"),
  ]) {
    setDraft(key, null)
  }
})

// A draft as an earlier page left it, at an age this one has to judge. Written
// to storage rather than staged by moving the clock: the record is what the
// cap reads, and the clock is not what these tests are about.
function storeAged(key: string, text: string, age: number): void {
  localStorage.setItem(key, JSON.stringify({ text, at: Date.now() - age }))
}

describe("a draft's key", () => {
  // The whole reason the kind is in the key: the description and the
  // conversation box are both about the same pull request, and typing in one
  // must not appear in the other.
  it("keeps two kinds about the same pull request apart", () => {
    setDraft(draftKey(PR, "body"), "a rewritten description")
    setDraft(draftKey(PR, "comment"), "a comment")

    expect(draftStore.get(draftKey(PR, "body"))).toBe("a rewritten description")
    expect(draftStore.get(draftKey(PR, "comment"))).toBe("a comment")
  })

  it("keeps one kind on two pull requests apart", () => {
    setDraft(draftKey(PR, "comment"), "about 389")
    setDraft(draftKey(OTHER, "comment"), "about 12")

    expect(draftStore.get(draftKey(PR, "comment"))).toBe("about 389")
    expect(draftStore.get(draftKey(OTHER, "comment"))).toBe("about 12")
  })

  it("keeps two projects reading the same numbers apart", () => {
    setDraft(draftKey(PR, "comment"), "mine")
    setDraft(draftKey({ projectId: OTHER_PROJECT, number: 389 }, "comment"), "theirs")

    expect(draftStore.get(draftKey(PR, "comment"))).toBe("mine")
  })

  // A thread id rides the end of the key, so it must not be able to land on the
  // description's or the comment's.
  it("cannot be forged out of a thread id", () => {
    expect(draftKey(PR, "reply", "body")).not.toBe(draftKey(PR, "body"))
    expect(draftKey(PR, "reply", THREAD)).not.toBe(draftKey(PR, "comment"))
  })
})

describe("a draft that has not been written", () => {
  it("reads as no draft, not as an empty one", () => {
    expect(draftStore.get(draftKey(PR, "reply", THREAD))).toBeNull()
  })

  // The distinction the reply box and the description both hang on: a box that
  // is open and empty is still open, and a description being cleared is a
  // legitimate edit. Collapsing the two would close a box under the user.
  it("is not the same as a draft that was emptied", () => {
    setDraft(draftKey(PR, "body"), "")

    expect(draftStore.get(draftKey(PR, "body"))).toBe("")
    expect(draftStore.get(draftKey(PR, "body"))).not.toBeNull()
  })
})

describe("subscribers", () => {
  it("hear their own key and no other", () => {
    let body = 0
    let comment = 0
    const offBody = draftStore.subscribe(draftKey(PR, "body"), () => {
      body++
    })
    const offComment = draftStore.subscribe(draftKey(PR, "comment"), () => {
      comment++
    })

    setDraft(draftKey(PR, "body"), "typing")

    expect(body).toBe(1)
    expect(comment).toBe(0)
    offBody()
    offComment()
  })

  // Every keystroke re-renders the box it was typed into; a keystroke that
  // changed nothing must not re-render anything, since the reply box lives
  // inside a CodeMirror widget that redraws with it.
  it("are not woken by a write that changes nothing", () => {
    setDraft(draftKey(PR, "body"), "same")
    let woken = 0
    const off = draftStore.subscribe(draftKey(PR, "body"), () => {
      woken++
    })

    setDraft(draftKey(PR, "body"), "same")

    expect(woken).toBe(0)
    off()
  })
})

describe("a draft across a reload", () => {
  it("comes back as it was typed", async () => {
    setDraft(draftKey(PR, "reply", THREAD), "half a reply")

    const next = await reload()

    expect(next.draftStore.get(next.draftKey(PR, "reply", THREAD))).toBe("half a reply")
  })

  it("is gone once it has been sent", async () => {
    setDraft(draftKey(PR, "comment"), "a comment")
    setDraft(draftKey(PR, "comment"), null)

    const next = await reload()

    expect(next.draftStore.get(next.draftKey(PR, "comment"))).toBeNull()
    expect(localStorage.getItem(draftKey(PR, "comment"))).toBeNull()
  })

  // Emptying the box is the other way a draft ends. In memory "" and null stay
  // apart — the box is still open — but neither is prose worth restoring.
  it("is gone once the box has been emptied", async () => {
    setDraft(draftKey(PR, "body"), "a rewritten description")
    setDraft(draftKey(PR, "body"), "")

    const next = await reload()

    expect(next.draftStore.get(next.draftKey(PR, "body"))).toBeNull()
    expect(localStorage.getItem(draftKey(PR, "body"))).toBeNull()
  })

  // A pref must never be able to break a launch: a record from another build,
  // or a hand-edited one, restores nothing and is collected on the way past.
  it("is dropped when the stored record cannot be read", async () => {
    localStorage.setItem(draftKey(PR, "comment"), "{not json")

    const next = await reload()

    expect(next.draftStore.get(next.draftKey(PR, "comment"))).toBeNull()
    expect(localStorage.getItem(draftKey(PR, "comment"))).toBeNull()
  })
})

describe("the sweep against the open pull requests", () => {
  it("collects a draft whose pull request is no longer open", async () => {
    setDraft(draftKey(PR, "comment"), "about a merged one")
    setDraft(draftKey(OTHER, "comment"), "about an open one")

    sweepDrafts(PROJECT, [OTHER.number, NEWEST])

    const next = await reload()
    expect(next.draftStore.get(next.draftKey(PR, "comment"))).toBeNull()
    expect(next.draftStore.get(next.draftKey(OTHER, "comment"))).toBe("about an open one")
  })

  it("collects every draft on that pull request, replies included", () => {
    setDraft(draftKey(PR, "body"), "a description")
    setDraft(draftKey(PR, "reply", THREAD), "a reply")

    sweepDrafts(PROJECT, [NEWEST])

    expect(localStorage.getItem(draftKey(PR, "body"))).toBeNull()
    expect(localStorage.getItem(draftKey(PR, "reply", THREAD))).toBeNull()
  })

  // The list is one project's. Another project's pull request numbering says
  // nothing about this one's, and sweeping across them would collect drafts
  // for pull requests that are open.
  it("leaves another project's drafts alone", () => {
    const theirs = draftKey({ projectId: OTHER_PROJECT, number: 389 }, "comment")
    setDraft(theirs, "another project's")

    sweepDrafts(PROJECT, [NEWEST])

    expect(localStorage.getItem(theirs)).not.toBeNull()
  })

  // The box on screen is not emptied under the user: a pull request merged
  // while a reply was being written still has that reply in the page.
  it("does not take what is still on screen", () => {
    setDraft(draftKey(PR, "comment"), "still being typed")

    sweepDrafts(PROJECT, [NEWEST])

    expect(draftStore.get(draftKey(PR, "comment"))).toBe("still being typed")
  })
})

describe("the sweep against a list that is behind", () => {
  // Numbers are monotonic, so a pull request opened after the list was read is
  // above everything in it — and the screen can be painting from an answer
  // minutes old. Its absence from the list is the list being stale, never the
  // pull request having closed.
  it("keeps a draft on a number the list could not have carried", () => {
    const opened = draftKey({ projectId: PROJECT, number: NEWEST + 1 }, "comment")
    setDraft(opened, "about one opened since")

    sweepDrafts(PROJECT, [OTHER.number, NEWEST])

    expect(localStorage.getItem(opened)).not.toBeNull()
  })

  // With no rows there is no ceiling, and so no evidence about any number: a
  // stale empty answer must not read as "this repository has nothing open".
  // Those drafts are the age cap's.
  it("keeps everything when the list came back empty", () => {
    setDraft(draftKey(PR, "comment"), "about something")

    sweepDrafts(PROJECT, [])

    expect(localStorage.getItem(draftKey(PR, "comment"))).not.toBeNull()
  })
})

describe("the age cap", () => {
  it("collects a draft nobody came back to", async () => {
    storeAged(draftKey(PR, "comment"), "written a long time ago", DRAFT_MAX_AGE_MS + 1)

    const next = await reload()

    expect(next.draftStore.get(next.draftKey(PR, "comment"))).toBeNull()
    expect(localStorage.getItem(draftKey(PR, "comment"))).toBeNull()
  })

  it("keeps one that is only nearly that old", async () => {
    storeAged(draftKey(PR, "comment"), "written a while ago", DRAFT_MAX_AGE_MS - 60_000)

    const next = await reload()

    expect(next.draftStore.get(next.draftKey(PR, "comment"))).toBe("written a while ago")
  })

  // The sweep is the other place the cap is enforced: a project left open for
  // weeks collects the leftovers of the ones nobody opens, which is the whole
  // point of a backstop under a per-project list.
  it("is enforced by the sweep too, across every project", () => {
    const theirs = draftKey({ projectId: OTHER_PROJECT, number: 7 }, "comment")
    storeAged(theirs, "another project's, and old", DRAFT_MAX_AGE_MS + 1)

    sweepDrafts(PROJECT, [])

    expect(localStorage.getItem(theirs)).toBeNull()
  })
})

describe("a stored record", () => {
  // A record with no timestamp cannot be aged, and a draft that cannot be aged
  // is a draft nothing would ever collect.
  it("that carries no time reads as expired", () => {
    expect(parseDraft(JSON.stringify({ text: "orphaned" }), Date.now())).toBeNull()
  })

  it("that carries no prose reads as no draft", () => {
    expect(parseDraft(JSON.stringify({ text: "", at: Date.now() }), Date.now())).toBeNull()
  })
})
