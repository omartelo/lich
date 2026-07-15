import { useEffect, useState } from "react"
import { Store } from "@/lib/rpc"
import { Input } from "@/components/ui/input"
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
      <Input
        value={bin}
        onChange={(event) => persist(event.target.value)}
        placeholder="claude"
        spellCheck={false}
        aria-label="Claude Code custom path"
        className="w-96 max-w-full font-mono"
      />
    </SettingBlock>
  )
}
