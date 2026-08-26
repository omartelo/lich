// @vitest-environment jsdom
//
// The caching half of useRemoteResource, mounted for real. The rest of the
// suite runs in node against pure modules, and remote-cache.test.ts covers the
// store on its own — but what this feature is about happens between two mounts
// of a component, and no pure test can see that: a screen the user walked away
// from has to come back painted, and the refetch has to run anyway.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { StrictMode, createElement } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { clearRemoteCache } from "@/lib/remote-cache"
import { useRemoteResource } from "@/lib/use-remote-resource"

const KEY = "/repo 388 abc"
const CACHE = "probe /repo 388"

interface Frame {
  data: string
  loading: boolean
  error: string | null
}

// Every frame the hook produced, so a mount can be read as a sequence rather
// than as a final value: what the user saw on the way to the answer is the
// whole question here.
//
// Mounted inside StrictMode, as the app runs (main.tsx). That is not
// decoration — React discards and replays a render that updates state during
// it, and the first version of this hook lost its seeded answer only under
// that replay. A probe mounted bare called the bug green.
function probe(frames: Frame[], load: () => Promise<string>, cache?: string, key = KEY) {
  function Probe() {
    const { data, loading, error } = useRemoteResource(key, load, { empty: "", cache })
    frames.push({ data, loading, error })
    return null
  }
  return createElement(StrictMode, null, createElement(Probe))
}

/** The distinct states a mount passed through, in order, as readable strings. */
function states(frames: Frame[]): string[] {
  const seen = frames.map((f) => `${JSON.stringify(f.data)} loading=${f.loading} error=${f.error}`)
  return seen.filter((state, i) => state !== seen[i - 1])
}

const ANSWERED = '"answer" loading=false error=null'

beforeEach(() => {
  clearRemoteCache()
})

describe("a remembered lookup", () => {
  it("paints the last answer on the next mount and refetches underneath", async () => {
    const frames: Frame[] = []
    let calls = 0
    const load = () => {
      calls++
      return Promise.resolve("answer")
    }

    const first = await mountBudget(probe(frames, load, CACHE))
    await first.act(() => {})
    expect(states(frames)).toEqual(['"" loading=true error=null', ANSWERED])
    await first.unmount()
    const asked = calls

    frames.length = 0
    const second = await mountBudget(probe(frames, load, CACHE))
    await second.act(() => {})

    // One state for the whole mount: the answer was there from the first frame
    // and never left it. A seed laid down and then dropped by a replayed render
    // is the bug this pins, and it shows up here as an extra blank state.
    expect(states(frames)).toEqual([ANSWERED])
    // Stale-while-revalidate, not a hit that skips the call: what is on screen
    // is the old answer exactly until the new one lands. How many calls that is
    // is StrictMode's business — it double-invokes every effect — so what is
    // asserted is that the request was made again at all.
    expect(calls).toBeGreaterThan(asked)
    await second.unmount()
  })

  // The reason the cache key is not `key`. `key` carries what dates an answer —
  // on the pull request screen, the checkout's HEAD, which a fresh mount does
  // not have until its git poll answers. A cache keyed by it would miss on
  // exactly the frame it exists for.
  it("paints an answer filed under a key that has since moved", async () => {
    const frames: Frame[] = []
    const load = () => Promise.resolve("answer")

    const first = await mountBudget(probe(frames, load, CACHE, "/repo 388 abc"))
    await first.act(() => {})
    await first.unmount()

    frames.length = 0
    // The same request, with the marker not yet arrived — the shape of a remount.
    const second = await mountBudget(probe(frames, load, CACHE, "/repo 388 "))
    await second.act(() => {})

    expect(states(frames)).toEqual([ANSWERED])
    await second.unmount()
  })

  it("starts blank and loading when the caller keeps no answers", async () => {
    const frames: Frame[] = []
    const load = () => Promise.resolve("answer")

    const first = await mountBudget(probe(frames, load))
    await first.act(() => {})
    await first.unmount()

    frames.length = 0
    const second = await mountBudget(probe(frames, load))
    await second.act(() => {})

    expect(states(frames)).toEqual(['"" loading=true error=null', ANSWERED])
    await second.unmount()
  })

  // The answer is filed before the sequence guard drops it, so a lookup the user
  // walked out on still pays for the return trip.
  it("keeps an answer that arrived after the component was gone", async () => {
    const frames: Frame[] = []
    let land: (value: string) => void = () => {}
    const load = () =>
      new Promise<string>((resolve) => {
        land = resolve
      })

    const first = await mountBudget(probe(frames, load, CACHE))
    await first.unmount()
    land("late")
    await new Promise((resolve) => setTimeout(resolve, 0))

    frames.length = 0
    const second = await mountBudget(probe(frames, load, CACHE))

    expect(states(frames)).toEqual(['"late" loading=false error=null'])
    await second.unmount()
  })

  // A lookup that failed says nothing about the last one that worked, so the
  // answer stays filed and the mount after it is painted again.
  it("leaves the filed answer standing when a refetch fails", async () => {
    const frames: Frame[] = []
    const answer = () => Promise.resolve("answer")
    const fail = () => Promise.reject(new Error("gh: not found"))

    const first = await mountBudget(probe(frames, answer, CACHE))
    await first.act(() => {})
    await first.unmount()

    frames.length = 0
    const second = await mountBudget(probe(frames, fail, CACHE))
    await second.act(() => {})
    expect(frames[frames.length - 1]?.error).toContain("not found")
    await second.unmount()

    frames.length = 0
    const third = await mountBudget(probe(frames, answer, CACHE))
    await third.act(() => {})

    expect(states(frames)).toEqual([ANSWERED])
    await third.unmount()
  })
})
