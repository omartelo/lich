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
import { SessionLaunchMenuItems } from "./SessionLaunchMenuItems"
import { WorktreeScriptRows } from "./WorktreeScriptRows"

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
  setup.run = ""
  setup.script = ""
  setup.saved = []
})

const text = () => document.body.textContent ?? ""

function menu(onRun?: () => void) {
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
        onNewSession: () => {},
        onRun,
      }),
    ),
  )
}

test("the launch menu offers Run only when the project ships a run script", async () => {
  const without = await mountBudget(menu())
  expect(text()).toContain("New Terminal")
  expect(text()).not.toContain("Run")
  await without.unmount()

  const clicks: number[] = []
  const with_ = await mountBudget(menu(() => clicks.push(1)))
  expect(text()).toContain("Run")

  const item = [...document.querySelectorAll('[role="menuitem"]')].find(
    (element) => element.textContent === "Run",
  )
  if (!item) {
    throw new Error("Run item not rendered")
  }
  await with_.act(() => {
    ;(item as HTMLElement).click()
  })
  expect(clicks).toEqual([1])
  await with_.unmount()
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
