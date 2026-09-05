import { GitBranch, Play, Terminal } from "lucide-react"
import { ProviderIcon } from "@/components/ProviderIcon"
import {
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu"
import type { ProviderState } from "@/lib/providers-store"
import type { ProviderKind } from "@/lib/session/sessions"

interface WorktreeMenuAction {
  disabled: boolean
  onSelect: () => void
}

interface SessionLaunchMenuItemsProps {
  providers: ProviderState[]
  terminalLabel: "Terminal" | "New Terminal"
  onNewSession: (kind: ProviderKind | "shell") => void
  worktree?: WorktreeMenuAction
  /** Open the checkout's Run card. Absent when the project ships no
   * .lich/run-worktree.sh — there would be no command to run. */
  onRun?: () => void
}

export function SessionLaunchMenuItems({
  providers,
  terminalLabel,
  onNewSession,
  worktree,
  onRun,
}: SessionLaunchMenuItemsProps) {
  return (
    <>
      <DropdownMenuGroup>
        {providers.map((provider) => (
          <DropdownMenuItem key={provider.id} onClick={() => onNewSession(provider.id)}>
            <ProviderIcon kind={provider.id} />
            {provider.name}
          </DropdownMenuItem>
        ))}
      </DropdownMenuGroup>
      <DropdownMenuSeparator />
      <DropdownMenuGroup>
        <DropdownMenuItem onClick={() => onNewSession("shell")}>
          <Terminal />
          {terminalLabel}
        </DropdownMenuItem>
        {onRun && (
          <DropdownMenuItem onClick={onRun}>
            <Play />
            Run
          </DropdownMenuItem>
        )}
        {worktree && (
          <DropdownMenuItem disabled={worktree.disabled} onClick={worktree.onSelect}>
            <GitBranch />
            Worktree
          </DropdownMenuItem>
        )}
      </DropdownMenuGroup>
    </>
  )
}
