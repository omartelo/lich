// @vitest-environment jsdom
//
// Render budgets: how much of the tree repaints for one thing happening.
//
// lich keeps every session's card in the sidebar and every session's terminal
// mounted at once, so the cost of a background session reporting itself is paid
// by the whole window. These pin that cost. Each budget is asserted whole, with
// toEqual and an exact count per component: an upper bound would also pass when
// a renamed component measures nothing at all, and the invariant here is as much
// about the names that must be absent — the other cards, the sidebar — as about
// the ones present.
//
// The tree measured starts below the providers: ProjectsProvider is replaced by
// its context value, because standing the real one up means faking its whole
// boot sequence of RPC calls. So a regression that made *the provider* re-render
// on every status event would repaint everything and pass here — read a green
// run as "the sidebar and the terminals are isolated", not as "nothing above
// them can repaint the window".
//
// The harness has to be imported before anything that reaches react-dom, which
// is why it is first here (see @/test/render-budget).
import { mountBudget } from "@/test/render-budget"
import { createElement, useEffect } from "react"
import { HashRouter } from "react-router-dom"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import { ProjectsContext, type ProjectsValue } from "@/providers/projects-context"
import { STATUS_EVENT, USAGE_EVENT } from "@/lib/session/session-events"

// The /events channel, as the app sees it: the stores subscribe at import, and a
// test publishes to the same registry the backend socket would. Mocked rather
// than driven through app-events itself because the real module opens a
// WebSocket and reconnects forever when it cannot.
const bus = vi.hoisted(() => {
  const handlers = new Map<string, Set<(data: unknown) => void>>()
  return {
    on(name: string, callback: (data: unknown) => void) {
      let set = handlers.get(name)
      if (!set) {
        set = new Set()
        handlers.set(name, set)
      }
      set.add(callback)
      return () => {
        handlers.get(name)?.delete(callback)
      }
    },
    emit(name: string, data: unknown) {
      for (const callback of handlers.get(name) ?? []) {
        callback(data)
      }
    },
  }
})

vi.mock("@/lib/app-events", () => ({ onAppEvent: bus.on, dispatchEnvelope: () => {} }))

// Every backend call hangs, which is what a component should render for: the git
// status, the pull request and the provider roster all stay at their loading
// branch, and none of them lands mid-budget as a repaint nobody asked for. The
// two spawn checks are the exception — a terminal that never clears them never
// mounts, and the mounted terminals are the point of the last test.
const never = () => new Promise<never>(() => {})
vi.mock("@/lib/rpc", () => {
  const hangs = new Proxy({}, { get: () => never })
  return {
    endpoint: () => ({ base: "http://localhost", token: "token" }),
    Terminal: new Proxy(
      {},
      {
        get: (_target, name) =>
          name === "WorkdirMissing" || name === "ResumeAvailable" ? async () => false : never,
      },
    ),
    DropService: hangs,
    ProjectService: hangs,
    Store: hangs,
    Fonts: hangs,
    AgentPlugin: hangs,
    AppUpdate: hangs,
    PatchNotes: hangs,
    System: hangs,
    Providers: hangs,
    Quota: hangs,
    Themes: hangs,
  }
})

// The terminal is canvas and WebGL, which jsdom cannot measure honestly, so it
// stands in as a component that records its own mounts. That counter answers the
// question a render budget cannot: a re-render and a remount look alike from
// outside, and only one of them throws away a running PTY.
const terminalMounts = vi.hoisted(() => ({ counts: {} as Record<string, number> }))
vi.mock("@/components/TerminalView", () => ({
  TerminalView: ({ sessionId }: { sessionId: string }) => {
    useEffect(() => {
      terminalMounts.counts[sessionId] = (terminalMounts.counts[sessionId] ?? 0) + 1
    }, [sessionId])
    return null
  },
}))

// jsdom reports every element at zero, and the terminal stage lays its panes out
// from its own measured width — unmeasured, it draws the focused session alone.
// So the observer answers with a window big enough for the grid to be a grid;
// which layout that produces is panes.test.ts's business, not this file's.
const STAGE = { width: 1200, height: 800 }
class FakeResizeObserver {
  constructor(private readonly callback: ResizeObserverCallback) {}
  observe() {
    this.callback(
      [{ contentRect: STAGE } as unknown as ResizeObserverEntry],
      this as unknown as ResizeObserver,
    )
  }
  unobserve() {}
  disconnect() {}
}
vi.stubGlobal("ResizeObserver", FakeResizeObserver)

// HashRouter follows history, not the hash property, so a test moves the route
// the way the browser's back button does.
const navigate = (to: string) => {
  window.location.hash = `#${to}`
  window.dispatchEvent(new PopStateEvent("popstate", { state: null }))
}

const noop = () => {}

// Two projects, so the last test has somewhere to switch to, and three sessions
// in the first, so "this card and nothing else" has two cards to be wrong about.
const workspace: ProjectsValue = {
  projects: [
    { id: "p1", name: "repo", path: "/repo" },
    { id: "p2", name: "other", path: "/other" },
  ],
  sessions: {
    p1: {
      sessions: [
        { id: "s1", label: "one", kind: "claude" },
        { id: "s2", label: "two", kind: "claude" },
        { id: "s3", label: "three", kind: "claude" },
      ],
      activeId: "s1",
      nextSeq: 4,
    },
    p2: { sessions: [{ id: "s4", label: "four", kind: "claude" }], activeId: "s4", nextSeq: 2 },
  },
  homeId: null,
  openProject: async () => {},
  openRecent: async () => true,
  ensureHomeProject: async () => "p1",
  closeProject: noop,
  newSession: () => "",
  newWorktreeSession: () => "",
  reopenWorktreeSession: async () => "",
  resumeClosedSession: async () => {},
  closeSession: noop,
  discardSession: noop,
  keepSession: noop,
  activateSession: noop,
  renameSession: noop,
  setEntrypoint: noop,
  scheduleSession: noop,
  pinSession: noop,
  reorderProjects: noop,
  reorderSessions: noop,
}

const usageEvent = (id: string) => ({
  id,
  percent: 10,
  tokens: 100,
  window: 1_000,
  model: "model",
  effort: "",
})

// What the cost-only rung reports: oh-my-pi, opencode and Crush record what a
// turn spent and no window to take it against, so every context field arrives
// zeroed (docs/ceilings.md).
const costOnlyUsageEvent = (id: string) => ({
  id,
  percent: 0,
  tokens: 0,
  window: 0,
  model: "",
  effort: "",
  costUsd: 0.25,
})

// A busy card counts how long it has been busy, on a shared 1s clock (see
// session-age). Left running it lands a repaint of its own in the middle of a
// budget, so the clock is frozen. Only the timers: faking the microtask queue
// along with them stops React committing at all.
beforeEach(() => {
  vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval"] })
})
afterEach(() => {
  vi.useRealTimers()
  // The split is a pref, so a test that opens one would otherwise hand the next
  // one a stage already divided.
  localStorage.clear()
})

async function mountSidebar() {
  window.location.hash = "#/projects/p1"
  const { SessionSidebar } = await import("./sidebar/SessionSidebar")
  const { FooterBar } = await import("./FooterBar")
  const { SettingsProvider } = await import("@/providers/settings")
  const budget = await mountBudget(
    createElement(
      HashRouter,
      null,
      createElement(
        SettingsProvider,
        null,
        createElement(
          ProjectsContext.Provider,
          { value: workspace },
          createElement(SessionSidebar, { onCollapse: noop }),
          createElement(FooterBar, { dock: null, onDock: noop }),
        ),
      ),
    ),
  )
  // The mount is not a budget; what one card costs after it is.
  budget.take()
  return budget
}

// What one card drags along when it repaints: its own chrome, and the base-ui
// context-menu and tooltip parts wrapped around it. The unnamed pair are
// base-ui's own anonymous render functions.
const CARD_CHROME = {
  ContextMenu: 1,
  ContextMenuRoot: 1,
  ContextMenuTrigger: 2,
  ContextMenuContent: 1,
  MenuRoot: 1,
  MenuPortal: 1,
  FloatingTree: 1,
  Tooltip: 1,
  TooltipRoot: 1,
  TooltipTrigger: 2,
  TooltipContent: 1,
  TooltipPortal: 1,
  SessionTooltip: 1,
  SessionStatusIcon: 1,
  ProviderIcon: 1,
  BrandIcon: 1,
  CloseButton: 1,
  Pin: 1,
  X: 1,
  Anonymous: 2,
}

test("a status event repaints that session's card and nothing else", async () => {
  const budget = await mountSidebar()
  await budget.act(() => bus.emit(STATUS_EVENT, { id: "s2", state: "busy" }))
  // s2 is a background card: the sidebar, the two cards beside it and the footer
  // are all absent, which is the invariant.
  expect(budget.take()).toEqual({ "SessionCard#s2": 1, ...CARD_CHROME })
  await budget.unmount()
})

test("a usage event for a background session repaints nothing", async () => {
  const budget = await mountSidebar()
  await budget.act(() => bus.emit(USAGE_EVENT, usageEvent("s3")))
  // No card reads usage — only the footer does, and it follows the active
  // session. A background session's token count is free.
  expect(budget.take()).toEqual({})
  await budget.unmount()
})

test("a usage event for the active session repaints the footer, not the sidebar", async () => {
  const budget = await mountSidebar()
  await budget.act(() => bus.emit(USAGE_EVENT, usageEvent("s1")))
  // The whole footer re-renders for a token count, segments that know nothing
  // about usage included — that is the measured cost, not an endorsement of it.
  // No SessionCard appears, which is what this pins.
  expect(budget.take()).toEqual({
    FooterBar: 1,
    FooterButton: 2,
    ContextRing: 1,
    SessionModel: 1,
    PlanQuota: 1,
    ProviderIcon: 1,
    BrandIcon: 1,
    Separator: 4,
    Tooltip: 4,
    TooltipRoot: 4,
    TooltipTrigger: 8,
    TooltipContent: 4,
    TooltipPortal: 4,
    Paperclip: 1,
    Folder: 1,
    Code: 1,
    Anonymous: 3,
  })
  await budget.unmount()
})

test("a usage event with no context window draws no ring", async () => {
  const budget = await mountSidebar()
  await budget.act(() => bus.emit(USAGE_EVENT, costOnlyUsageEvent("s1")))
  // The footer still repaints, but no ContextRing and no provider glyph: a
  // report with no window has no percentage to draw a ring around, and the model
  // slot renders nothing rather than naming zero of zero tokens. Against the
  // budget above, the missing ContextRing, ProviderIcon, BrandIcon and one
  // Separator are the whole degradation to the cost-only rung. (The cost figure
  // itself is not here: its setting is an RPC, and every RPC hangs in this rig.)
  expect(budget.take()).toEqual({
    FooterBar: 1,
    FooterButton: 2,
    SessionModel: 1,
    PlanQuota: 1,
    Separator: 2,
    Tooltip: 3,
    TooltipRoot: 3,
    TooltipTrigger: 6,
    TooltipContent: 3,
    TooltipPortal: 3,
    Paperclip: 1,
    Folder: 1,
    Code: 1,
    Anonymous: 3,
  })
  await budget.unmount()
})

test("switching project re-renders the mounted terminals but never remounts one", async () => {
  window.location.hash = "#/projects/p1"
  const { TerminalHost } = await import("./TerminalHost")
  const budget = await mountBudget(
    createElement(
      HashRouter,
      null,
      createElement(
        ProjectsContext.Provider,
        { value: workspace },
        createElement(TerminalHost, null),
      ),
    ),
  )
  budget.take()
  expect(terminalMounts.counts).toEqual({ s1: 1 })

  // Two commits: the route moves, then p2's session clears its spawn gate and
  // mounts. Three TerminalView renders across them — s1 in both, s4 in the
  // second — because nothing under TerminalHost is memoized.
  await budget.act(() => navigate("/projects/p2"))
  expect(budget.take()).toEqual({
    HashRouter: 1,
    Router: 1,
    TerminalHost: 2,
    TerminalView: 3,
    ResumeSessionDialog: 2,
    WorktreeCloseDialogs: 2,
    RunningSessionDialog: 2,
    CloseWorktreeDialog: 2,
    ForceRemoveWorktreeDialog: 2,
    ConfirmDialog: 8,
    Dialog: 8,
    DialogRoot: 8,
    DialogContent: 8,
    DialogPortal: 16,
  })
  expect(terminalMounts.counts).toEqual({ s1: 1, s4: 1 })

  // Coming back is one commit, both terminals already spawned — and both of them
  // re-render for it.
  await budget.act(() => navigate("/projects/p1"))
  expect(budget.take()).toEqual({
    HashRouter: 1,
    Router: 1,
    TerminalHost: 1,
    TerminalView: 2,
    ResumeSessionDialog: 1,
    WorktreeCloseDialogs: 1,
    RunningSessionDialog: 1,
    CloseWorktreeDialog: 1,
    ForceRemoveWorktreeDialog: 1,
    ConfirmDialog: 4,
    Dialog: 4,
    DialogRoot: 4,
    DialogContent: 4,
    DialogPortal: 8,
  })
  // The point of the whole test: two round trips, one mount each. A terminal
  // that remounted would have thrown away its PTY and its scrollback.
  expect(terminalMounts.counts).toEqual({ s1: 1, s4: 1 })
  await budget.unmount()
})

test("adding a pane mounts its terminal and never remounts the ones already up", async () => {
  window.location.hash = "#/projects/p1"
  const { TerminalHost } = await import("./TerminalHost")
  const { writeGroups } = await import("@/lib/session/panes-store")
  const budget = await mountBudget(
    createElement(
      HashRouter,
      null,
      createElement(
        ProjectsContext.Provider,
        { value: workspace },
        createElement(TerminalHost, null),
      ),
    ),
  )
  budget.take()
  // A delta rather than the whole map: the mounts counter is module state shared
  // with the tests above, and what this pins is what *this* action did to it.
  const before = { ...terminalMounts.counts }

  await budget.act(() =>
    writeGroups("p1", [{ id: "g1", name: "wall", cells: ["s1", "s2"], cols: [], rows: [] }]),
  )

  // The second pane's terminal is born once, and the first pane's is not thrown
  // away and rebuilt around it — a split that re-keyed its layers would kill the
  // very PTY it was opened to watch, and a remount is indistinguishable from a
  // re-render without this counter.
  expect(terminalMounts.counts).toEqual({ ...before, s2: (before.s2 ?? 0) + 1 })
  // And the stage really divided: two cells drawn side by side on a 1200px
  // stage, with a column seam between them. The node-environment gate cannot
  // render, so this file is where the stage's own chrome gets to prove it draws
  // at all.
  expect(document.querySelectorAll("[data-pane]")).toHaveLength(2)
  expect(document.querySelector('[aria-label="Resize the columns"]')).not.toBeNull()
  await budget.unmount()
})
