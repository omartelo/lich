import { beforeEach, describe, expect, it } from "vitest"
import { draftKey, draftStore } from "./draft-store"

const PR = "https://github.com/omartelo/lich/pull/389"
const OTHER = "https://github.com/omartelo/lich/pull/12"

const KEYS = [
  draftKey("body", PR),
  draftKey("comment", PR),
  draftKey("reply", PR),
  draftKey("body", OTHER),
  draftKey("comment", OTHER),
  draftKey("reply", "PRRT_kwDOabc"),
]

beforeEach(() => {
  for (const key of KEYS) {
    draftStore.set(key, null)
  }
})

describe("a draft's key", () => {
  // The whole reason the kind is in the key: the description and the
  // conversation box are both about the same pull request, and typing in one
  // must not appear in the other.
  it("keeps two kinds about the same pull request apart", () => {
    draftStore.set(draftKey("body", PR), "a rewritten description")
    draftStore.set(draftKey("comment", PR), "a comment")

    expect(draftStore.get(draftKey("body", PR))).toBe("a rewritten description")
    expect(draftStore.get(draftKey("comment", PR))).toBe("a comment")
  })

  it("keeps one kind on two pull requests apart", () => {
    draftStore.set(draftKey("comment", PR), "about 389")
    draftStore.set(draftKey("comment", OTHER), "about 12")

    expect(draftStore.get(draftKey("comment", PR))).toBe("about 389")
    expect(draftStore.get(draftKey("comment", OTHER))).toBe("about 12")
  })

  // A separator that could appear inside an id would let one id be written that
  // lands on another kind's key. A newline appears in neither a URL nor a
  // GitHub node id, and this is what says so out loud.
  it("cannot be forged out of an id", () => {
    expect(draftKey("body", PR)).not.toBe(draftKey("comment", PR))
    expect(draftKey("reply", `${PR}\ncomment`)).not.toBe(draftKey("comment", PR))
  })
})

describe("a draft that has not been written", () => {
  it("reads as no draft, not as an empty one", () => {
    expect(draftStore.get(draftKey("reply", "PRRT_kwDOabc"))).toBeNull()
  })

  // The distinction the reply box and the description both hang on: a box that
  // is open and empty is still open, and a description being cleared is a
  // legitimate edit. Collapsing the two would close a box under the user.
  it("is not the same as a draft that was emptied", () => {
    draftStore.set(draftKey("body", PR), "")

    expect(draftStore.get(draftKey("body", PR))).toBe("")
    expect(draftStore.get(draftKey("body", PR))).not.toBeNull()
  })
})

describe("subscribers", () => {
  it("hear their own key and no other", () => {
    let body = 0
    let comment = 0
    const offBody = draftStore.subscribe(draftKey("body", PR), () => {
      body++
    })
    const offComment = draftStore.subscribe(draftKey("comment", PR), () => {
      comment++
    })

    draftStore.set(draftKey("body", PR), "typing")

    expect(body).toBe(1)
    expect(comment).toBe(0)
    offBody()
    offComment()
  })

  // Every keystroke re-renders the box it was typed into; a keystroke that
  // changed nothing must not re-render anything, since the reply box lives
  // inside a CodeMirror widget that redraws with it.
  it("are not woken by a write that changes nothing", () => {
    draftStore.set(draftKey("body", PR), "same")
    let woken = 0
    const off = draftStore.subscribe(draftKey("body", PR), () => {
      woken++
    })

    draftStore.set(draftKey("body", PR), "same")

    expect(woken).toBe(0)
    off()
  })
})
