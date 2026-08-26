import type { ReactElement } from "react"

// A render budget: how many times each component's body ran for one action.
//
// React DOM only reports commits to a tree it believes is being profiled, and
// it decides that once, when it is first imported, by looking for the DevTools
// global hook. So the shim below is installed at module scope and react-dom is
// pulled in lazily inside mountBudget — which means this module has to be
// imported before anything else that reaches react-dom. `hook.renderers` is the
// proof that it was: react-dom calls inject() on the way in, and mountBudget
// refuses to measure a tree that never did.
//
// Counting is React's own answer, not an approximation: a fiber whose component
// body ran in a commit carries the PerformedWork flag. Fibers React skipped
// wholesale keep their flags from an earlier commit, so they are excluded by
// identity — a fiber object reused from the previous committed tree was not
// visited at all.

const PERFORMED_WORK = 0b1

interface Fiber {
  key: string | null
  type: unknown
  elementType: unknown
  flags: number
  child: Fiber | null
  sibling: Fiber | null
}

interface DevToolsHook {
  renderers: Map<number, unknown>
  supportsFiber: true
  inject(renderer: unknown): number
  onCommitFiberRoot(id: number, root: { current: Fiber }): void
  onCommitFiberUnmount(): void
  onPostCommitFiberRoot(): void
}

const commits: Fiber[] = []

const hook: DevToolsHook = {
  renderers: new Map(),
  supportsFiber: true,
  inject(renderer) {
    const id = hook.renderers.size + 1
    hook.renderers.set(id, renderer)
    return id
  },
  onCommitFiberRoot(_id, root) {
    commits.push(root.current)
  },
  onCommitFiberUnmount() {},
  onPostCommitFiberRoot() {},
}

const globals = globalThis as Record<string, unknown>
globals.__REACT_DEVTOOLS_GLOBAL_HOOK__ = hook
// React's own flag for "act() is legal here"; without it every act call warns.
globals.IS_REACT_ACT_ENVIRONMENT = true
// jsdom ships no ResizeObserver and measures every element at zero, so the
// components that watch their own width (a card's path ellipsis, the terminal's
// fit) have nothing to observe here anyway. A stub that never fires is the
// honest stand-in: it keeps the mount from throwing without inventing a size.
if (!globals.ResizeObserver) {
  globals.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  }
}
// Same story for matchMedia, which the settings provider asks about the OS
// colour scheme: jsdom has no answer, and a budget does not depend on one.
if (!window.matchMedia) {
  window.matchMedia = (media: string) =>
    ({
      media,
      matches: false,
      onchange: null,
      addEventListener() {},
      removeEventListener() {},
      addListener() {},
      removeListener() {},
      dispatchEvent: () => false,
    }) as MediaQueryList
}

// componentName is what a budget is written against, so it answers with the
// name in the source: the wrapped function for memo and forwardRef, never the
// wrapper. An anonymous component would be indistinguishable from the next one,
// so it is named after nothing and shows up as such.
function componentName(type: unknown): string {
  if (typeof type === "function") {
    const fn = type as { displayName?: string; name?: string }
    return fn.displayName || fn.name || "Anonymous"
  }
  if (type && typeof type === "object") {
    const wrapper = type as { displayName?: string; render?: unknown; type?: unknown }
    if (wrapper.displayName) {
      return wrapper.displayName
    }
    if (wrapper.render) {
      return componentName(wrapper.render)
    }
    if (wrapper.type) {
      return componentName(wrapper.type)
    }
  }
  return "Anonymous"
}

// A keyed element is one of several siblings of the same type, which is exactly
// the case a per-session budget has to tell apart — so the key rides in the
// name rather than collapsing three cards into "SessionCard: 3".
function fiberLabel(fiber: Fiber): string {
  const name = componentName(fiber.type ?? fiber.elementType)
  return fiber.key === null ? name : `${name}#${fiber.key}`
}

export interface RenderBudget {
  /** Renders since the last take(), by component. Empty means nothing repainted. */
  take(): Record<string, number>
  /** Run something that updates the tree and settle every render it causes. */
  act(run: () => void | Promise<void>): Promise<void>
  unmount(): Promise<void>
}

export async function mountBudget(element: ReactElement): Promise<RenderBudget> {
  const { act } = await import("react")
  const { createRoot } = await import("react-dom/client")
  if (hook.renderers.size === 0) {
    throw new Error(
      "react-dom was imported before render-budget; move its import to the top of the test file",
    )
  }

  const container = document.createElement("div")
  document.body.appendChild(container)
  const root = createRoot(container)

  // The fibers of the last committed tree. A fiber still in this set was reused
  // untouched by the next commit, so whatever its flags say happened earlier.
  let previous = new Set<Fiber>()
  // Commits are collected globally, so a budget starts reading where the tree
  // it owns begins — never at another root's history.
  let read = commits.length

  const take = (): Record<string, number> => {
    const counts: Record<string, number> = {}
    // Every commit since the last read, so an action that settles in two passes
    // is billed for both.
    for (; read < commits.length; read++) {
      const current = new Set<Fiber>()
      const walk = (start: Fiber | null): void => {
        for (let fiber = start; fiber !== null; fiber = fiber.sibling) {
          current.add(fiber)
          if (!previous.has(fiber) && (fiber.flags & PERFORMED_WORK) !== 0) {
            const label = fiberLabel(fiber)
            counts[label] = (counts[label] ?? 0) + 1
          }
          walk(fiber.child)
        }
      }
      walk(commits[read])
      previous = current
    }
    return counts
  }

  // An update can be raised from a promise the action only started — a spawn
  // gate answering, a fetch landing — so every act is followed by an empty one:
  // it drains the microtasks the first act left pending and commits whatever
  // they scheduled, inside act, rather than after the budget was read.
  const settle = async (run: () => void | Promise<void>): Promise<void> => {
    await act(async () => await run())
    await act(async () => {})
  }

  await settle(() => {
    root.render(element)
  })

  return {
    take,
    act: settle,
    unmount: async () => {
      await act(async () => {
        root.unmount()
      })
      container.remove()
    },
  }
}
