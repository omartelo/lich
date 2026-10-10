// @vitest-environment jsdom
//
// A folder gathers cards from different checkouts, so a card's own pull request
// entry has to open that card's checkout: the Pulls screen reviews whatever
// session is active, and activating the folder's other card shows the wrong PR.
// The card it parks belongs to that checkout too: a folder has no checkout of
// its own, and keying it on the project root showed the root's pull request.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { SessionSidebar } from "./SessionSidebar"
import { SettingsProvider } from "@/providers/settings"
import { createElement } from "react"
import { HashRouter } from "react-router-dom"
import { expect, test, vi } from "vitest"
import { ProjectsContext, type ProjectsValue } from "@/providers/projects-context"
import { closePulls, isPullsOpen } from "@/lib/pulls-card-store"

vi.mock("@/lib/app-events", () => ({ onAppEvent: () => () => {}, dispatchEnvelope: () => {} }))

const never = () => new Promise<never>(() => {})
vi.mock("@/lib/rpc", () => {
  const hangs = new Proxy({}, { get: () => never })
  return {
    endpoint: () => ({ base: "http://localhost", token: "token" }),
    Terminal: hangs,
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

const noop = () => {}

function workspace(activated: string[]): ProjectsValue {
  return {
    projects: [{ id: "p1", name: "repo", path: "/repo" }],
    sessions: {
      p1: {
        sessions: [
          { id: "s1", label: "one", kind: "claude", path: "/wt/one", folder: "work" },
          { id: "s2", label: "two", kind: "claude", path: "/wt/two", folder: "work" },
        ],
        activeId: "s1",
        nextSeq: 3,
      },
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
    activateSession: (_projectId, sessionId) => activated.push(sessionId),
    renameSession: noop,
    setEntrypoint: noop,
    scheduleSession: noop,
    pinSession: noop,
    colorSessions: noop,
    fileSessions: noop,
    renameSessionFolder: noop,
    reorderProjects: noop,
    reorderSessions: noop,
  }
}

test("a card's pull request entry opens that card's checkout, not the active one", async () => {
  window.location.hash = "#/projects/p1"
  const activated: string[] = []
  const mounted = await mountBudget(
    createElement(
      HashRouter,
      null,
      createElement(
        SettingsProvider,
        null,
        createElement(
          ProjectsContext.Provider,
          { value: workspace(activated) },
          createElement(SessionSidebar, { onCollapse: noop }),
        ),
      ),
    ),
  )

  const card = [...document.querySelectorAll("*")].find(
    (element) => element.textContent === "two" && element.children.length === 0,
  )
  if (!card) {
    throw new Error("card two not rendered")
  }
  await mounted.act(() => {
    card.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, clientX: 1, clientY: 1 }))
  })
  const item = [...document.querySelectorAll('[role="menuitem"]')].find(
    (element) => element.textContent === "Pull request",
  ) as HTMLElement | undefined
  if (!item) {
    throw new Error("Pull request item not rendered")
  }
  await mounted.act(() => {
    item.click()
  })

  expect(activated).toEqual(["s2"])
  expect(window.location.hash).toBe("#/projects/p1/pulls")
  expect(isPullsOpen("/wt/two")).toBe(true)
  expect(isPullsOpen("/repo")).toBe(false)

  // The parked card reopens the same checkout, though s1 is still the active one.
  const parked = [...document.querySelectorAll("button")].find((element) =>
    element.textContent?.startsWith("Pull request"),
  )
  if (!parked) {
    throw new Error("parked Pull request card not rendered")
  }
  await mounted.act(() => {
    parked.click()
  })
  expect(activated).toEqual(["s2", "s2"])
  closePulls("/wt/two")
  await mounted.unmount()
})
