import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { createSessionStatusStore } from "./session-status-store"

// A stand-in for the /events subscription: hands back an emit so a test can
// drive the store the way the backend would.
function fakeSource() {
  let handler: (data: unknown) => void = () => {}
  const source = vi.fn((h: (data: unknown) => void) => {
    handler = h
    return () => {}
  })
  return { source, emit: (data: unknown) => handler(data) }
}

const report = (id: string, state: string) => ({ id, state })

describe("createSessionStatusStore", () => {
  it("subscribes to the source once at creation, before any listener", () => {
    const { source } = fakeSource()
    createSessionStatusStore(source)
    expect(source).toHaveBeenCalledTimes(1)
  })

  it("records a status for a session nobody is subscribed to", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    expect(store.get("s1")).toBe("busy")
  })

  // The regression: switching projects unmounts the card, which unsubscribes.
  // The status — including one reported while it was away — must still be there
  // when it comes back.
  it("keeps the status across unsubscribe and resubscribe", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const off = store.subscribe("s1", () => {})
    emit(report("s1", "busy"))
    off()
    expect(store.get("s1")).toBe("busy")

    const notify = vi.fn()
    store.subscribe("s1", notify)
    expect(store.get("s1")).toBe("busy")
    emit(report("s1", "done"))
    expect(store.get("s1")).toBe("done")
    expect(notify).toHaveBeenCalledTimes(1)
  })

  it("applies a status reported while nothing was subscribed", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const off = store.subscribe("s1", () => {})
    off()
    emit(report("s1", "waiting"))
    expect(store.get("s1")).toBe("waiting")
  })

  it("notifies only the listeners of the session that changed", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const first = vi.fn()
    const second = vi.fn()
    store.subscribe("s1", first)
    store.subscribe("s2", second)
    emit(report("s1", "busy"))
    expect(first).toHaveBeenCalledTimes(1)
    expect(second).not.toHaveBeenCalled()
    expect(store.get("s2")).toBeNull()
  })

  it("skips the notify when the reported state repeats", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const notify = vi.fn()
    store.subscribe("s1", notify)
    emit(report("s1", "busy"))
    emit(report("s1", "busy"))
    expect(notify).toHaveBeenCalledTimes(1)
  })

  it("clears the status on idle and on an unknown state", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    emit(report("s1", "idle"))
    expect(store.get("s1")).toBeNull()

    emit(report("s2", "busy"))
    emit(report("s2", "from-a-newer-plugin"))
    expect(store.get("s2")).toBeNull()
  })

  it("ignores a malformed payload, keeping the previous status", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    emit({ id: "s1" })
    emit({ id: "s1", state: 42 })
    emit({ state: "done" })
    emit(null)
    emit("busy")
    expect(store.get("s1")).toBe("busy")
  })

  it("reports null for a session it has never heard of", () => {
    const { source } = fakeSource()
    const store = createSessionStatusStore(source)
    expect(store.get("ghost")).toBeNull()
  })
})

describe("pendingOf", () => {
  it("badges nothing for a project with no reported session", () => {
    const { source } = fakeSource()
    const store = createSessionStatusStore(source)
    expect(store.pendingOf([])).toBeNull()
    expect(store.pendingOf(["ghost"])).toBeNull()
  })

  it("ranks waiting over busy over done", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "done"))
    expect(store.pendingOf(["s1", "s2", "s3"])).toBe("done")
    emit(report("s2", "busy"))
    expect(store.pendingOf(["s1", "s2", "s3"])).toBe("busy")
    emit(report("s3", "waiting"))
    expect(store.pendingOf(["s1", "s2", "s3"])).toBe("waiting")
  })

  it("only counts the sessions of the project asked about", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("mine", "busy"))
    emit(report("theirs", "waiting"))
    expect(store.pendingOf(["mine"])).toBe("busy")
  })

  // Leaving a project marks its sessions seen, so the turn that finished while
  // the user was in there does not badge the tab they just left.
  it("drops a done once seen, and keeps live states regardless", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "done"))
    store.markSeen("s1")
    expect(store.pendingOf(["s1"])).toBeNull()
    expect(store.get("s1")).toBe("done") // the card still shows its check

    emit(report("s2", "busy"))
    store.markSeen("s2")
    expect(store.pendingOf(["s2"])).toBe("busy")

    // A prompt left unanswered is still blocking after you walk away.
    emit(report("s3", "waiting"))
    store.markSeen("s3")
    expect(store.pendingOf(["s3"])).toBe("waiting")
  })

  it("badges a done that lands after the session was marked seen", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    store.markSeen("s1")
    emit(report("s1", "done"))
    expect(store.pendingOf(["s1"])).toBe("done")
  })

  it("notifies subscribers when a seen done stops badging", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const notify = vi.fn()
    store.subscribe("s1", notify)
    emit(report("s1", "done"))
    notify.mockClear()
    store.markSeen("s1")
    expect(notify).toHaveBeenCalledTimes(1)
    store.markSeen("s1")
    expect(notify).toHaveBeenCalledTimes(1) // already seen: nothing changed
  })

  it("does not notify when marking a live state seen", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const notify = vi.fn()
    store.subscribe("s1", notify)
    emit(report("s1", "busy"))
    notify.mockClear()
    store.markSeen("s1")
    expect(notify).not.toHaveBeenCalled()
  })

  it("ignores markSeen for a session it has never heard of", () => {
    const { source } = fakeSource()
    const store = createSessionStatusStore(source)
    expect(() => store.markSeen("ghost")).not.toThrow()
  })
})

describe("runningOf", () => {
  it("returns nothing when no session of the project is mid-turn", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "done"))
    expect(store.runningOf([])).toEqual([])
    expect(store.runningOf(["s1", "ghost"])).toEqual([])
  })

  it("counts both a working agent and one blocked on the user", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    emit(report("s2", "waiting"))
    emit(report("s3", "done"))
    expect(store.runningOf(["s1", "s2", "s3"])).toEqual(["s1", "s2"])
  })

  it("only counts the sessions of the project asked about", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("mine", "busy"))
    emit(report("theirs", "busy"))
    expect(store.runningOf(["mine"])).toEqual(["mine"])
  })

  // Unlike a tab badge, a running turn is not something the user can dismiss by
  // looking at it: the guard must still fire for a session already seen.
  it("keeps counting a running session that was marked seen", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    store.markSeen("s1")
    expect(store.runningOf(["s1"])).toEqual(["s1"])
  })

  it("stops counting a session once its turn ends", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    emit(report("s1", "done"))
    expect(store.runningOf(["s1"])).toEqual([])
  })
})

describe("since", () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(1_000)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it("stamps the moment the status changed", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "waiting"))
    expect(store.since("s1")).toBe(1_000)
  })

  // The whole point of the readout: the hook keeps reporting the same state
  // while a turn runs, and the card must say how long the session has been
  // waiting, not how long since the last report said so again.
  it("does not restart on a repeat of the same state", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "waiting"))
    vi.setSystemTime(21_000)
    emit(report("s1", "waiting"))
    expect(store.since("s1")).toBe(1_000)
  })

  it("keeps the stamp of a status that has not changed since the app started", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    vi.setSystemTime(1_000 + 90 * 60_000)
    expect(store.since("s1")).toBe(1_000)
  })

  it("restarts the clock on a real transition", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    vi.setSystemTime(5_000)
    emit(report("s1", "waiting"))
    expect(store.since("s1")).toBe(5_000)
  })

  // Entries outlive their listeners, so a status that arrived while the card
  // was unmounted is timed from when it arrived, not from when the card came
  // back and re-subscribed.
  it("stamps a status that arrives while nothing is subscribed", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const off = store.subscribe("s1", () => {})
    off()
    emit(report("s1", "waiting"))
    vi.setSystemTime(9_000)
    store.subscribe("s1", () => {})
    expect(store.since("s1")).toBe(1_000)
  })

  it("has no clock for a session with no status", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    expect(store.since("ghost")).toBeNull()
    store.subscribe("s1", () => {})
    expect(store.since("s1")).toBeNull()
    emit(report("s2", "busy"))
    emit(report("s2", "idle"))
    expect(store.since("s2")).toBeNull()
  })
})

describe("pendingAll / subscribeAll", () => {
  it("starts empty", () => {
    const { source } = fakeSource()
    const store = createSessionStatusStore(source)
    expect(store.pendingAll()).toEqual([])
  })

  it("queues waiting and done, but never busy", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "waiting"))
    emit(report("s2", "busy"))
    emit(report("s3", "done"))
    expect(store.pendingAll()).toEqual([
      { id: "s1", status: "waiting" },
      { id: "s3", status: "done" },
    ])
  })

  it("spans sessions from any project — it is a flat, global list", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("a", "waiting"))
    emit(report("b", "done"))
    expect(store.pendingAll().map((p) => p.id)).toEqual(["a", "b"])
  })

  it("drops a done once seen, but keeps a waiting the user walked away from", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("done1", "done"))
    emit(report("wait1", "waiting"))
    store.markSeen("done1")
    store.markSeen("wait1")
    expect(store.pendingAll()).toEqual([{ id: "wait1", status: "waiting" }])
  })

  it("re-queues a done that lands after the session was marked seen", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    emit(report("s1", "busy"))
    store.markSeen("s1")
    emit(report("s1", "done"))
    expect(store.pendingAll()).toEqual([{ id: "s1", status: "done" }])
  })

  it("notifies subscribers when the queue changes", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const notify = vi.fn()
    const off = store.subscribeAll(notify)
    emit(report("s1", "waiting"))
    expect(notify).toHaveBeenCalledTimes(1)
    off()
    emit(report("s2", "waiting"))
    expect(notify).toHaveBeenCalledTimes(1) // unsubscribed: no further calls
  })

  it("keeps a stable reference — and stays silent — when the queue is unchanged", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const notify = vi.fn()
    store.subscribeAll(notify)
    emit(report("s1", "waiting"))
    const first = store.pendingAll()
    notify.mockClear()
    // A busy report never touches the queue, so the reference must not change
    // and no re-render fires.
    emit(report("s2", "busy"))
    expect(store.pendingAll()).toBe(first)
    expect(notify).not.toHaveBeenCalled()
  })

  it("does not fire when a waiting is marked seen (waiting ignores seen)", () => {
    const { source, emit } = fakeSource()
    const store = createSessionStatusStore(source)
    const notify = vi.fn()
    store.subscribeAll(notify)
    emit(report("s1", "waiting"))
    notify.mockClear()
    store.markSeen("s1")
    expect(notify).not.toHaveBeenCalled()
    expect(store.pendingAll()).toEqual([{ id: "s1", status: "waiting" }])
  })
})
