// @vitest-environment jsdom
//
// The one thing useStoredSetting adds over the lookup underneath it: the pairing
// of the request with the answer filed for it. Settings draws every provider's
// section at the same position, so React keeps one instance and the key changes
// underneath it — and a hook that lets one provider's answer stand under
// another's request does not show a stale readout, it writes the wrong path into
// the wrong key on the next keystroke.
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { StrictMode, createElement, useLayoutEffect, useState } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { clearRemoteCache } from "@/lib/remote-cache"
import { useStoredSetting } from "@/lib/use-stored-setting"

const reads: string[] = []
const writes: string[] = []

vi.mock("@/lib/rpc", () => ({
  Store: {
    GetSetting: (key: string, scope: string) => {
      reads.push(`${key}@${scope}`)
      // "" is what the store answers for a key nobody has set, which is the
      // case the fallback exists for — not only the frame before it answers.
      if (key === "unset") {
        return Promise.resolve("")
      }
      // The same key answers differently per scope, which is the whole point of
      // a layered override.
      return Promise.resolve(scope ? `/${scope}/${key}` : `/usr/bin/${key}`)
    },
    SetSetting: (key: string, scope: string, value: string) => {
      writes.push(`${key}@${scope}=${value}`)
      return Promise.resolve(null)
    },
  },
}))

// Every frame the hook *committed*, so a mount reads as a sequence rather than
// as a final value: what stood on screen on the way to the answer is the whole
// question here.
//
// Recorded from a layout effect, never from the render body. A render that
// updates state during it runs again from its base state, and StrictMode runs
// every pass twice — so the body sees values that were never painted, and an
// assertion written against them reads a correct hook as an oscillation.
//
// Mounted inside StrictMode, as the app runs (main.tsx): React discards and
// replays a render that updates state during it, and the seeding this hook
// leans on is done during render.
function probe(frames: string[], fallback = "") {
  let move: (next: string) => void = () => {}
  let type: (next: string) => void = () => {}
  function Probe() {
    // The key changes on a *live* component, which is the only way this hazard
    // ever happens: a remount would start the lookup from scratch and never
    // exercise the move at all.
    const [provider, setProvider] = useState("claude")
    move = setProvider
    const [value, persist] = useStoredSetting(provider, "", fallback)
    type = persist
    useLayoutEffect(() => {
      frames.push(`${provider}:${value}`)
    })
    return null
  }
  return {
    element: createElement(StrictMode, null, createElement(Probe)),
    move: (next: string) => move(next),
    type: (next: string) => type(next),
  }
}

/** The distinct states a mount passed through, in order. */
function states(frames: string[]): string[] {
  return frames.filter((state, i) => state !== frames[i - 1])
}

beforeEach(() => {
  clearRemoteCache()
  reads.length = 0
  writes.length = 0
})

describe("a stored settings value", () => {
  it("never lets one key's answer stand under another's", async () => {
    const frames: string[] = []
    const { element, move } = probe(frames)

    const mounted = await mountBudget(element)
    await mounted.act(() => {})
    expect(states(frames)).toEqual(["claude:", "claude:/usr/bin/claude"])

    frames.length = 0
    await mounted.act(() => move("codex"))

    // Blank in the same render as the change, and only then the new answer:
    // "/usr/bin/claude" never appears beside "codex".
    expect(states(frames)).toEqual(["codex:", "codex:/usr/bin/codex"])
    await mounted.unmount()
  })

  it("paints a key it has already answered on the first frame back", async () => {
    const frames: string[] = []
    const { element, move } = probe(frames)

    const mounted = await mountBudget(element)
    await mounted.act(() => {})
    await mounted.act(() => move("codex"))
    frames.length = 0
    await mounted.act(() => move("claude"))

    // One state for the whole move: no blank between the two, because the
    // answer for this key was still on file.
    expect(states(frames)).toEqual(["claude:/usr/bin/claude"])
    await mounted.unmount()
  })

  it("brings the answer back to the next mount of the screen", async () => {
    const frames: string[] = []
    const first = await mountBudget(probe(frames).element)
    await first.act(() => {})
    await first.unmount()
    const asked = reads.length

    frames.length = 0
    const second = await mountBudget(probe(frames).element)
    await second.act(() => {})

    expect(states(frames)).toEqual(["claude:/usr/bin/claude"])
    // Stale-while-revalidate, not a hit that skips the read: what is on screen
    // is the old answer exactly until the new one lands.
    expect(reads.length).toBeGreaterThan(asked)
    await second.unmount()
  })

  // The half that makes the field usable while it is being typed in, and the
  // half that makes it safe: what was typed is tagged with the request it was
  // typed against, so moving to another provider drops it rather than showing
  // it — and rather than writing it into that provider's key.
  it("drops what was typed when the key moves under it", async () => {
    const frames: string[] = []
    const { element, move, type } = probe(frames)

    const mounted = await mountBudget(element)
    await mounted.act(() => {})
    await mounted.act(() => type("  /opt/claude  "))
    expect(frames[frames.length - 1]).toBe("claude:  /opt/claude  ")
    // Stored trimmed, under the key it was typed against.
    expect(writes).toEqual(["claude@=/opt/claude"])

    frames.length = 0
    await mounted.act(() => move("codex"))

    expect(states(frames)).toEqual(["codex:", "codex:/usr/bin/codex"])
    expect(writes).toEqual(["claude@=/opt/claude"])
    await mounted.unmount()
  })

  // ProviderBinary reads one key at two scopes at once — the project's override
  // and the global one — and draws them as two rows of the same block. They are
  // two questions, so the scope has to be part of both the request and the key
  // the answer is filed under.
  //
  // It takes two mounts to see a shared key go wrong, which is why this is not
  // the obvious one-mount assertion: each hook still runs its own load closure
  // with its own scope, so the first visit is right whatever the key says. The
  // damage is on the way back, where both layers seed from the one filed answer
  // and the block paints an override nobody typed — corrected a round-trip
  // later, which reads as a flicker rather than as a bug.
  it("keeps the layers of one key apart across a remount", async () => {
    const frames: string[] = []
    function Layers() {
      const [global] = useStoredSetting("claude", "")
      const [project] = useStoredSetting("claude", "proj-1")
      useLayoutEffect(() => {
        frames.push(`${global}|${project}`)
      })
      return null
    }
    const element = () => createElement(StrictMode, null, createElement(Layers))

    const first = await mountBudget(element())
    await first.act(() => {})
    expect(frames[frames.length - 1]).toBe("/usr/bin/claude|/proj-1/claude")
    expect(new Set(reads)).toEqual(new Set(["claude@", "claude@proj-1"]))
    await first.unmount()

    frames.length = 0
    const second = await mountBudget(element())
    await second.act(() => {})

    // Painted from the first visit's answers, each layer from its own.
    expect(states(frames)).toEqual(["/usr/bin/claude|/proj-1/claude"])
    await second.unmount()
  })

  // The fallback is what the value reads as before the store answers and after
  // it answers with nothing. These are the switches that confine a session and
  // hand an agent a credential: the unknown state is drawn as the answer lich
  // has always behaved like, never as the permissive one.
  it("reads as the fallback until the store answers", async () => {
    const frames: string[] = []
    const { element } = probe(frames, "off")

    const mounted = await mountBudget(element)
    await mounted.act(() => {})

    expect(states(frames)).toEqual(["claude:off", "claude:/usr/bin/claude"])
    await mounted.unmount()
  })

  it("keeps reading as the fallback once the store answers with nothing", async () => {
    const frames: string[] = []
    const { element, move } = probe(frames, "off")

    const mounted = await mountBudget(element)
    await mounted.act(() => {})
    frames.length = 0
    await mounted.act(() => move("unset"))

    // Never a frame of "", which the rung above this would have drawn as no
    // rung selected at all.
    expect(states(frames)).toEqual(["unset:off"])
    await mounted.unmount()
  })
})
