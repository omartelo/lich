import { useEffect, useState } from "react"
import { Service as Store } from "../../../bindings/github.com/skipodotdev/skipo/internal/store"
import { SettingBlock } from "./SettingBlock"

// Global scope for a setting: an empty project id. Per-project overrides live
// under the project's id and are surfaced elsewhere.
const GLOBAL_SCOPE = ""
const CLAUDE_BIN_KEY = "claude.bin"

export function ClaudeCodeSettings() {
  const [bin, setBin] = useState("")

  useEffect(() => {
    void Store.GetSetting(CLAUDE_BIN_KEY, GLOBAL_SCOPE).then(setBin)
  }, [])

  const persist = (value: string) => {
    setBin(value)
    void Store.SetSetting(CLAUDE_BIN_KEY, GLOBAL_SCOPE, value.trim())
  }

  return (
    <SettingBlock
      title="Custom path"
      description="Path to the Claude Code binary or a launcher script spawned in each terminal. Leave empty to run 'claude' from your $PATH."
    >
      <input
        value={bin}
        onChange={(event) => persist(event.target.value)}
        placeholder="claude"
        spellCheck={false}
        aria-label="Claude Code custom path"
        className="h-9 w-96 max-w-full rounded-md border border-border bg-background px-3 font-mono text-sm text-foreground outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring"
      />
    </SettingBlock>
  )
}
