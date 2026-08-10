import { FileText, Globe, ListTodo, Pencil, Search, Terminal, type LucideIcon } from "lucide-react"

// The glyph a tool family wears on the session card. Keyed by the names both
// harnesses actually report (tabled in docs/hooks/session-state.md) — Codex
// says "Bash" like Claude Code does, and `apply_patch` is its one own word.
// A Map rather than an object literal because the key arrives over the wire:
// an object would answer "constructor" with a function, which React would then
// try to render.
const GLYPHS = new Map<string, LucideIcon>([
  ["Bash", Terminal],
  ["Read", FileText],
  ["Edit", Pencil],
  ["Write", Pencil],
  ["NotebookEdit", Pencil],
  ["apply_patch", Pencil],
  ["Grep", Search],
  ["Glob", Search],
  ["WebFetch", Globe],
  ["WebSearch", Globe],
  ["TodoWrite", ListTodo],
])

// toolGlyph resolves a reported tool name to its glyph, or null for a name the
// map does not know — an MCP tool, one shipped after this build. The line reads
// fine without one, which is why an unknown name draws nothing rather than a
// placeholder.
export function toolGlyph(name: string): LucideIcon | null {
  return GLYPHS.get(name) ?? null
}
