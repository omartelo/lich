// @vitest-environment jsdom
//
// Smoke: the whole app, rendered as main.tsx renders it, walked through each
// screen a user reaches without a running agent. Each test asks one thing of a
// screen: that it renders its resting state without a throw, with the control
// that state exists to offer. Nothing about how it got there.
//
// What this catches is the class the logic suites cannot: a render path that
// type-checks and builds and then throws in the browser, such as a base-ui part
// mounted outside the parent it needs. React reports every caught throw through
// console.error (and so does ErrorBoundary), so a clean console is the verdict.
//
// What it does not reach: a session's terminal. xterm draws on a canvas jsdom
// does not have, so every scenario here is a workspace with no session open.
//
// Each test boots a fresh module graph: the stores are module-level, and one
// scenario's workspace must not be the next one's starting state.
import { act, createElement, StrictMode } from "react"
import { createRoot, type Root } from "react-dom/client"
import { afterEach, beforeEach, expect, test, vi } from "vitest"
import type { StoredProject } from "@/lib/api-types"
import { SETTING_SECTIONS } from "@/lib/settings-index"
import { type Answer, installSmokeBackend, type SmokeBackend } from "@/test/smoke-backend"
// Never rendered from here. Importing it once at file load transforms the app's
// whole module graph up front, so each boot's fresh import only evaluates it:
// left to the first test, the transform cost the 5s it is given on a busy
// machine, and its boot finished inside the tests after it.
import "./App"

// The version the fake backend reports as both running and latest, so no update
// prompt and no release notes stand over the screen under test.
const VERSION = "0.63.0"

// The Home tab every workspace holds from its first launch, here with its first
// shell closed: a session would mount a terminal, which this suite cannot draw.
const HOME: StoredProject = {
  id: "home",
  name: "dev",
  path: "/home/dev",
  nextSeq: 2,
  activeSessionId: "",
  defaultProvider: "",
  sessions: [],
}

const PROJECT: StoredProject = {
  id: "p1",
  name: "repo",
  path: "/work/repo",
  nextSeq: 1,
  activeSessionId: "",
  defaultProvider: "",
  sessions: [],
}

// A user past the first launch: an agent chosen and this release's notes seen.
const RETURNING: Record<string, string> = {
  "provider.default": "claude",
  "update.patchNotesSeen": VERSION,
}

interface Workspace {
  projects?: StoredProject[]
  settings?: Record<string, string>
}

// The answers of a backend on a machine with git, gh and Claude Code installed,
// holding the given workspace. A setting nobody wrote reads as "", as the Go
// store answers it.
function backendFor({ projects = [], settings = {} }: Workspace): Record<string, Answer> {
  const stored = new Map(Object.entries(settings))
  return {
    "store.LoadState": () => [HOME, ...projects],
    "store.RecentProjects": () => [],
    "store.ClosedProjectCount": () => 0,
    "store.GetSetting": ([key]) => stored.get(String(key)) ?? "",
    "store.SetSetting": ([key, , value]) => {
      stored.set(String(key), String(value))
    },
    "project.Home": () => ({ id: HOME.id, name: HOME.name, path: HOME.path }),
    "project.Missing": () => [],
    "providers.Detect": () => [
      {
        id: "claude",
        name: "Claude Code",
        binary: "claude",
        installed: true,
        path: "/usr/bin/claude",
        source: "path",
        docs: "",
      },
    ],
    "providers.Verify": ([bin]) => ({ path: `/usr/bin/${bin}`, status: "ok" }),
    "agentplugin.Status": () => [],
    // A clean checkout on its main branch, with no pull request behind it.
    "project.Diff": () => ({ files: 0, added: 0, deleted: 0, head: "4b825dc", branch: "main" }),
    "project.BaseStatus": () => null,
    "project.WorktreeSetup": () => ({ script: "", run: "" }),
    "project.ListCheckouts": ([path]) => [{ name: "main", path }],
    "project.BranchesOf": ([paths]) =>
      Object.fromEntries((paths as string[]).map((path) => [path, "main"])),
    "project.PullRequest": () => null,
    "project.PullRequestDetail": () => null,
    "project.ListPullRequests": () => [],
    "project.GitHubAccounts": () => ["dev"],
    "project.CommitIdentity": () => ({ name: "Dev", email: "dev@example.com", local: false }),
    "store.ClosedSessions": () => ({ sessions: [], total: 0, indexing: 0 }),
    "fonts.List": () => ["FiraCode Nerd Font Mono"],
    "system.SandboxBackend": () => "bubblewrap",
    "system.SSHAgentKeys": () => null,
    "system.Diagnostics": () => ({
      version: VERSION,
      platform: "linux/amd64",
      logPath: "/home/dev/.config/lich/lich.log",
    }),
    "project.Tree": () => ({ files: ["README.md"], cut: false, hidden: [] }),
    "project.DiffText": () => "",
    "quota.Plans": () => [],
    "appupdate.Status": () => ({
      currentVersion: VERSION,
      latestVersion: VERSION,
      updateAvailable: false,
      canSelfApply: false,
      releaseUrl: "",
      installCommand: "",
    }),
    "patchnotes.Current": () => ({ version: VERSION, highlights: null, groups: null }),
    "system.TakeUncleanExit": () => false,
    "themes.List": () => [],
    "themes.ListBroken": () => [],
  }
}

let host: HTMLDivElement
let root: Root | null
let backend: SmokeBackend
let errors: string[]

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  localStorage.clear()
  vi.resetModules()
  errors = []
  vi.spyOn(console, "error").mockImplementation((...args: unknown[]) => {
    errors.push(args.map(String).join(" "))
  })
  host = document.createElement("div")
  document.body.append(host)
  root = null
})

afterEach(() => {
  act(() => root?.unmount())
  host.remove()
  document.body.replaceChildren()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

async function boot(answers: Record<string, Answer>): Promise<void> {
  backend = installSmokeBackend(answers)
  const { default: App } = await import("./App")
  const mounted = createRoot(host)
  root = mounted
  await act(async () => {
    mounted.render(createElement(StrictMode, null, createElement(App)))
  })
  await settle()
}

// Lets the backend round trips a screen starts land, and the ones those answers
// start in turn: done once a pass goes by with no new call.
async function settle(): Promise<void> {
  let seen = -1
  while (seen !== backend.calls.length) {
    seen = backend.calls.length
    await act(() => new Promise((resolve) => setTimeout(resolve, 20)))
  }
}

async function go(hash: string): Promise<void> {
  window.location.hash = hash
  await settle()
}

async function press(control: HTMLElement): Promise<void> {
  await act(async () => control.click())
  await settle()
}

// Typed where a key lands in the browser: on whatever holds focus.
async function type(key: string, modifiers: KeyboardEventInit = {}): Promise<void> {
  const target = document.activeElement ?? document.body
  await act(async () => {
    target.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, ...modifiers }))
  })
  await settle()
}

// The control by the name a screen reader reads: its aria-label, else its text.
// Scoped when the same name sits elsewhere on screen: the footer's Notifications
// bell and the settings section of the same name.
function button(name: string, scope = ""): HTMLElement {
  const controls = `${scope} :is(button, [role=button], [role=tab])`.trim()
  const found = [...document.querySelectorAll<HTMLElement>(controls)].filter(
    (element) => (element.getAttribute("aria-label") ?? element.textContent?.trim()) === name,
  )
  expect(found, `a control named "${name}"`).not.toHaveLength(0)
  return found[0]
}

function expectClean(): void {
  expect(errors).toEqual([])
  expect(backend.unanswered).toEqual([])
}

test("a first launch asks which agents you use", async () => {
  await boot(backendFor({}))
  expect(document.querySelector("[role=dialog]")).not.toBeNull()
  expectClean()
})

test("an empty workspace offers to open a project", async () => {
  await boot(backendFor({ settings: RETURNING }))
  expect(document.querySelector("[role=dialog]")).toBeNull()
  button("Open project")
  expectClean()
})

test("a project with no session offers to start one", async () => {
  await boot(backendFor({ projects: [PROJECT], settings: RETURNING }))
  await go(`#/projects/${PROJECT.id}`)
  button("New session")
  expectClean()
})

test("the dock opens on the code and on the changes", async () => {
  await boot(backendFor({ projects: [PROJECT], settings: RETURNING }))
  await go(`#/projects/${PROJECT.id}`)
  await press(button("Browse code"))
  expect(document.querySelector("aside[aria-label='File browser']")).not.toBeNull()
  await press(button("Review changes"))
  expect(document.querySelector("aside[aria-label='Review changes']")).not.toBeNull()
  expectClean()
})

test("every settings section renders", async () => {
  await boot(backendFor({ projects: [PROJECT], settings: RETURNING }))
  await go(`#/projects/${PROJECT.id}/settings`)
  for (const section of SETTING_SECTIONS) {
    await press(button(section.label, "nav"))
    expect(document.querySelector("h1")?.textContent, section.id).toContain(section.label)
  }
  expectClean()
})

test("the pull request screens render with no pull request open", async () => {
  await boot(backendFor({ projects: [PROJECT], settings: RETURNING }))
  await go(`#/projects/${PROJECT.id}/pulls`)
  button("Open on GitHub")
  await go(`#/projects/${PROJECT.id}/pulls/all`)
  expect(document.querySelector("[aria-label='Filter pull requests']")).not.toBeNull()
  expectClean()
})

test("the command palette and the shortcuts list open from the keyboard", async () => {
  await boot(backendFor({ projects: [PROJECT], settings: RETURNING }))
  expect(document.querySelector("[role=dialog]")).toBeNull()
  await type("k", { ctrlKey: true })
  expect(document.querySelector("[role=dialog]"), "command palette").not.toBeNull()
  await type("Escape")
  expect(document.querySelector("[role=dialog]")).toBeNull()
  await type("/", { ctrlKey: true })
  expect(document.querySelector("[role=dialog]"), "shortcuts list").not.toBeNull()
  expectClean()
})
