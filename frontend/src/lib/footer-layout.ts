import type { FooterVisibility } from "./footer-prefs"
import { readPref, writePref } from "./prefs"

export const FOOTER_ITEMS = [
  { id: "attach", label: "Attach file", example: "Attach file" },
  { id: "files", label: "File explorer", example: "Files" },
  { id: "changes", label: "Changes", example: "3 · +10 −2" },
  { id: "pr", label: "Pull request", example: "PR #123" },
  { id: "checkout", label: "Branch", example: "feature/toolbar" },
  { id: "path", label: "Working directory", example: "~/project" },
  { id: "model", label: "Model", example: "Codex · GPT-6" },
  { id: "context", label: "Context window", example: "42%" },
  { id: "plan", label: "Plan usage", example: "5h 28%" },
  { id: "cost", label: "Cost", example: "$1.25" },
  { id: "handsOn", label: "Hands-on time", example: "12m" },
  { id: "clock", label: "Date & time", example: "Sep 6 · 14:30" },
] as const

export type FooterItem = (typeof FOOTER_ITEMS)[number]["id"]
export type FooterSide = "left" | "right"
export type FooterZone = FooterSide | "available"
export interface FooterLayout {
  left: FooterItem[]
  right: FooterItem[]
}

export const DEFAULT_FOOTER_LAYOUT: FooterLayout = {
  left: ["attach", "files", "changes", "pr"],
  right: ["checkout", "model", "context", "plan", "handsOn"],
}
const KEY = "lich.footer.layout"

export const isFooterItem = (id: string): id is FooterItem =>
  FOOTER_ITEMS.some((item) => item.id === id)
export const hasFooterItem = (layout: FooterLayout, id: FooterItem): boolean =>
  layout.left.includes(id) || layout.right.includes(id)

// Old visibility choices become an initial layout only. A saved empty side is
// intentional; unknown or repeated items cannot clone a control on reload.
export function parseFooterLayout(raw: string | null): FooterLayout | null {
  try {
    const value: unknown = JSON.parse(raw ?? "null")
    if (!value || typeof value !== "object") return null
    const { left, right } = value as Record<string, unknown>
    if (!Array.isArray(left) || !Array.isArray(right)) return null
    const seen = new Set<string>()
    const clean = (items: unknown[]): FooterItem[] =>
      items.filter((id): id is FooterItem => {
        if (typeof id !== "string" || !isFooterItem(id) || seen.has(id)) return false
        seen.add(id)
        return true
      })
    return { left: clean(left), right: clean(right) }
  } catch {
    return null
  }
}

export function resolveFooterLayout(
  stored: FooterLayout | null,
  visibility: FooterVisibility,
  context: boolean,
  cost: boolean,
): FooterLayout {
  // Cost's backend flag also controls pricing. A layout from another browser
  // profile must not silently turn that work back on when an unrelated item moves.
  if (stored)
    return hasFooterItem(stored, "cost") === cost
      ? stored
      : moveFooterItem(stored, "cost", cost ? "right" : "available")
  const flags = { ...visibility, context, cost }
  const readings = ["model", "context", "plan", "cost", "handsOn", "clock"] as const
  return {
    left: [...DEFAULT_FOOTER_LAYOUT.left],
    right: ["checkout", ...readings.filter((id) => flags[id])],
  }
}

export function footerZone(layout: FooterLayout, id: string): FooterZone {
  if (id === "left" || layout.left.includes(id as FooterItem)) return "left"
  if (id === "right" || layout.right.includes(id as FooterItem)) return "right"
  return "available"
}

// One move serves pointer drops and the keyboard/menu alternatives. Dropping
// on a side appends; on an item it takes that item's position.
export function moveFooterItem(layout: FooterLayout, id: string, over: string): FooterLayout {
  if (
    !isFooterItem(id) ||
    id === over ||
    (!isFooterItem(over) && !["left", "right", "available"].includes(over))
  )
    return layout
  const side = footerZone(layout, over)
  const index = side === "available" ? -1 : layout[side].indexOf(over as FooterItem)
  const next = {
    left: layout.left.filter((item) => item !== id),
    right: layout.right.filter((item) => item !== id),
  }
  if (side !== "available") next[side].splice(index < 0 ? next[side].length : index, 0, id)
  return JSON.stringify(next) === JSON.stringify(layout) ? layout : next
}

export const readFooterLayout = (): FooterLayout | null => parseFooterLayout(readPref(KEY))
export const writeFooterLayout = (layout: FooterLayout): void =>
  writePref(KEY, JSON.stringify(layout))
