import type { ComponentType } from "react"
import { ClaudeCode, Codex, OpenCode } from "@lobehub/icons"
import { Sparkles, Terminal } from "lucide-react"
import type { SessionKind } from "./sessions"

// Brand marks per session kind. @lobehub/icons covers Claude/Codex/opencode as
// monochrome (currentColor) logos, so they inherit the surrounding text color
// and read on both themes. Crush has no lobehub icon yet — it borrows a lucide
// glyph until a real mark is dropped in. Shell sessions show a terminal.
// lobehub icons and lucide both type size as string | number.
type SizedIcon = ComponentType<{ size?: number | string }>

const BRAND: Partial<Record<SessionKind, SizedIcon>> = {
  claude: ClaudeCode,
  codex: Codex,
  opencode: OpenCode,
  crush: Sparkles,
  shell: Terminal,
}

// ProviderIcon renders the brand mark for a session kind. Size defaults to 16
// (lucide's size-4) for menus; cards pass a smaller size to sit beside labels.
export function ProviderIcon({ kind, size = 16 }: { kind: SessionKind; size?: number }) {
  const Icon = BRAND[kind] ?? Terminal
  return <Icon size={size} />
}
