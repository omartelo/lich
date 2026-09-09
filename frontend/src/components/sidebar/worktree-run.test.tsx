// @vitest-environment jsdom
//
// The Run card's two surfaces, mounted for real: the frontend gate is otherwise
// node-only, so a component that throws on render passes a green suite. These
// render the actual base-ui menu and the dialog's script rows.
//
// The harness is imported first for the reason render-budget.test.tsx names: it
// has to hook react-dom before anything else reaches it.
import { mountBudget } from "@/test/render-budget"
import { createElement } from "react"
import { beforeEach, expect, test, vi } from "vitest"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { RUN_NOT_ON_WINDOWS, SessionLaunchMenuItems } from "./SessionLaunchMenuItems"
import { WorktreeScriptRows } from "./WorktreeScriptRows"

// The OS the menu reads, behind a getter so one file can mount both sides: the
// component reads it per render, and vi.mock replaces the module for the whole
// file.
const os = vi.hoisted(() => ({ windows: false }))

vi.mock("@/lib/platform", () => ({
  get isWindows() {
    return os.windows
  },
  get isMac() {
    return false
  },
}))

const setup = vi.hoisted(() => ({
  run: "",
  script: "",
  saved: [] as Array<[string, string]>,
}))

vi.mock("@/lib/rpc", () => ({
  ProjectService: {
    WorktreeSetup: () => Promise.resolve({ script: setup.script, run: setup.run }),
    SaveWorktreeSetup: (path: string, value: string) => {
      setup.saved.push([path, value])
      setup.script = value
      return Promise.resolve(null)
    },
    SaveWorktreeRun: (path: string, value: string) => {
      setup.saved.push([path, value])
      setup.run = value
      return Promise.resolve(null)
    },
  },
}))

beforeEach(() => {
  os.windows = false
  setup.run = ""
  setup.script = ""
  setup.saved = []
})

const text = () => document.body.textContent ?? ""

function menu(run?: { open: boolean; onSelect: () => void }) {
  return createElement(
    DropdownMenu,
    { open: true },
    createElement(DropdownMenuTrigger, null, "+"),
    createElement(
      DropdownMenuContent,
      null,
      createElement(SessionLaunchMenuItems, {
        providers: [],
        terminalLabel: "New Terminal" as const,
        projectId: "p1",
        onNewSession: () => {},
        run,
      }),
    ),
  )
}

// The item to click, by the label it is wearing.
const menuItem = (label: string) =>
  [...document.querySelectorAll('[role="menuitem"]')].find(
    (element) => element.textContent === label,
  ) as HTMLElement | undefined

test("the launch menu offers Run only when the project ships a run script", async () => {
  const without = await mountBudget(menu())
  expect(text()).toContain("New Terminal")
  expect(text()).not.toContain("Run")
  await without.unmount()

  const clicks: number[] = []
  const with_ = await mountBudget(menu({ open: false, onSelect: () => clicks.push(1) }))
  expect(text()).toContain("Run")
  expect(text()).not.toContain("Go to Run card")

  const item = menuItem("Run")
  if (!item) {
    throw new Error("Run item not rendered")
  }
  await with_.act(() => {
    item.click()
  })
  expect(clicks).toEqual([1])
  await with_.unmount()
})

// One Run card per checkout (internal/spawn.Run): once the checkout has one, the
// same item goes to that card instead of asking for a second.
test("the item reads Go to Run card once the checkout has one", async () => {
  const focused: number[] = []
  const open = await mountBudget(menu({ open: true, onSelect: () => focused.push(1) }))
  expect(text()).toContain("Go to Run card")

  const item = menuItem("Go to Run card")
  if (!item) {
    throw new Error("Go to Run card item not rendered")
  }
  await open.act(() => {
    item.click()
  })
  expect(focused).toEqual([1])
  await open.unmount()
})

test("the dialog offers a run command when the repository ships none", async () => {
  const rows = await mountBudget(createElement(WorktreeScriptRows, { projectPath: "/src/lich" }))
  expect(text()).toContain("No run command")

  const set = [...document.querySelectorAll("button")].find(
    (element) => element.textContent === "Set",
  )
  if (!set) {
    throw new Error("Set button not rendered")
  }
  await rows.act(() => {
    set.click()
  })

  const field = document.querySelector<HTMLTextAreaElement>("#worktree-run")
  if (!field) {
    throw new Error("run editor not opened")
  }
  expect(text()).toContain(".lich/run-worktree.sh")
  await rows.unmount()
})

test("a configured run command is shown beside the setup script", async () => {
  setup.script = "pnpm install"
  setup.run = "pnpm dev --port $LICH_WORKTREE_PORT"
  const rows = await mountBudget(createElement(WorktreeScriptRows, { projectPath: "/src/lich" }))

  expect(text()).toContain("pnpm install")
  expect(text()).toContain("pnpm dev --port $LICH_WORKTREE_PORT")
  expect(text()).toContain(".lich/setup-worktree.sh")
  expect(text()).toContain(".lich/run-worktree.sh")
  await rows.unmount()
})

// The run script is one file, versioned and shared by every checkout, and it
// holds sh — so on Windows the row stays and says so, rather than going missing
// with no way to tell a withheld offer from an absent one.
test("the Run item is dead on Windows, naming why", async () => {
  os.windows = true
  const clicks: number[] = []
  const mounted = await mountBudget(menu({ open: false, onSelect: () => clicks.push(1) }))

  const row = [...document.querySelectorAll('[role="menuitem"]')].find((element) =>
    element.textContent?.startsWith("Run"),
  ) as HTMLElement | undefined
  if (!row) {
    throw new Error("Run row not rendered")
  }
  expect(row.getAttribute("data-disabled")).not.toBeNull()
  expect(row.textContent).toContain(RUN_NOT_ON_WINDOWS)

  await mounted.act(() => {
    row.click()
  })
  expect(clicks).toEqual([])
  await mounted.unmount()
})

test("the Run item is live everywhere else, with no reason to give", async () => {
  const clicks: number[] = []
  const mounted = await mountBudget(menu({ open: false, onSelect: () => clicks.push(1) }))

  const row = menuItem("Run")
  if (!row) {
    throw new Error("Run row not rendered")
  }
  expect(row.getAttribute("data-disabled")).toBeNull()
  await mounted.act(() => {
    row.click()
  })
  expect(clicks).toEqual([1])
  await mounted.unmount()
})
