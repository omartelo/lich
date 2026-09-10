import { useState } from "react"
import { ChevronLeft, GitBranch, Play, Terminal } from "lucide-react"
import { ProviderIcon } from "@/components/ProviderIcon"
import {
  DropdownMenuCheckboxItem,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu"
import { isWindows } from "@/lib/platform"
import type { ProviderState } from "@/lib/providers-store"
import { sandboxDefaultFor } from "@/lib/providers-store"
import { CONFINED_MEANS } from "@/lib/sandbox-copy"
import type { ProviderKind } from "@/lib/session/sessions"
import type { SandboxAnswer } from "@/lib/use-sandbox-choice"
import { useSandboxAsk } from "@/lib/use-sandbox-rung"

interface WorktreeMenuAction {
  disabled: boolean
  onSelect: () => void
}

/** The checkout's Run entry. open is whether it already has a Run card (one per
 * checkout, see internal/spawn.Run), which is what turns the item from opening
 * one into going to the one that is there. */
export interface RunMenuAction {
  open: boolean
  onSelect: () => void
}

interface SessionLaunchMenuItemsProps {
  providers: ProviderState[]
  terminalLabel: "Terminal" | "New Terminal"
  /** The project whose sandbox rung the confinement question is read from. */
  projectId: string
  /** sandbox is the confinement answer for the new session — "on"/"off" when
   * the rung asked for one, "" when it answered by itself. */
  onNewSession: (kind: ProviderKind | "shell", sandbox: SandboxAnswer) => void
  worktree?: WorktreeMenuAction
  /** The checkout's Run entry. Absent when the project ships no
   * .lich/run-worktree.sh — there would be no command to run. */
  run?: RunMenuAction
}

/** Why the Run item is dead on Windows: .lich/run-worktree.sh holds sh and a
 * session there runs PowerShell, which would take its lines as commands of its
 * own and expand $LICH_WORKTREE_PORT to nothing. */
export const RUN_NOT_ON_WINDOWS = "Run scripts are sh; not run on Windows."

// The checkout's Run row: live where the script can run, dead under the
// sentence naming why on Windows — SessionForkItem's idiom. The row stays
// either way, because a user who runs a card on Linux and finds nothing on
// Windows has no way to learn that the offer was withheld rather than missing.
function RunMenuItem({ run }: { run: RunMenuAction }) {
  if (isWindows) {
    return (
      <DropdownMenuItem disabled>
        <Play />
        <span className="flex flex-col items-start">
          Run
          <span className="text-xs">{RUN_NOT_ON_WINDOWS}</span>
        </span>
      </DropdownMenuItem>
    )
  }
  return (
    <DropdownMenuItem onClick={run.onSelect}>
      <Play />
      {run.open ? "Go to Run card" : "Run"}
    </DropdownMenuItem>
  )
}

interface SandboxStepProps {
  provider: ProviderState
  onOpen: (sandbox: SandboxAnswer) => void
  onBack: () => void
}

// SandboxStep is the "Ask each time" rung's question, put where the session is
// being opened from rather than in a dialog over it: the menu that was going to
// open the card holds the choice instead, and the item that opens it is one row
// below the box.
//
// A menu-native checkbox rather than the dialog's — the same question, the same
// wording — because a plain input inside a base-ui popup takes no part in the
// menu's roving focus, and a question only a mouse can answer is not one.
//
// It stands per provider because the rung does (store.sandboxKey): a menu
// listing Claude on Ask beside Codex on Everywhere cannot carry one box that
// means both.
function SandboxStep({ provider, onOpen, onBack }: SandboxStepProps) {
  // The rung's own answer, which for "ask" is the confined side and does not
  // depend on the checkout — so the side this session lands on is not asked
  // here (store.SandboxDefault).
  const [confined, setConfined] = useState(() => sandboxDefaultFor("ask", false))
  return (
    <>
      <DropdownMenuGroup>
        <DropdownMenuLabel className="flex items-center gap-2">
          <ProviderIcon kind={provider.id} />
          {provider.name}
        </DropdownMenuLabel>
        <DropdownMenuCheckboxItem checked={confined} onCheckedChange={setConfined}>
          Run confined
        </DropdownMenuCheckboxItem>
        <p className="px-2 pt-0.5 pb-1.5 text-xs text-muted-foreground">{CONFINED_MEANS}</p>
      </DropdownMenuGroup>
      <DropdownMenuSeparator />
      <DropdownMenuGroup>
        <DropdownMenuItem onClick={() => onOpen(confined ? "on" : "off")}>
          <ProviderIcon kind={provider.id} />
          Open session
        </DropdownMenuItem>
        {/* Back stays in the menu — closing it is what Escape and the trigger
            already do, and a "Back" that dismisses the whole thing is a click
            nobody gets a second try at. */}
        <DropdownMenuItem closeOnClick={false} onClick={onBack}>
          <ChevronLeft />
          Back
        </DropdownMenuItem>
      </DropdownMenuGroup>
    </>
  )
}

export function SessionLaunchMenuItems({
  providers,
  terminalLabel,
  projectId,
  onNewSession,
  worktree,
  run,
}: SessionLaunchMenuItemsProps) {
  const asking = useSandboxAsk(
    providers.map((provider) => provider.id),
    projectId,
  )
  // The provider whose confinement is being asked about, null while the menu is
  // its usual list. Every other rung answers for itself, so its item opens the
  // card on the click as it always has.
  const [pending, setPending] = useState<ProviderState | null>(null)

  if (pending) {
    return (
      <SandboxStep
        provider={pending}
        // Put back before the card opens, rather than left to the popup
        // unmounting with the menu: the next click on this menu has to be a
        // provider list, never last time's half-answered question.
        onOpen={(sandbox) => {
          setPending(null)
          onNewSession(pending.id, sandbox)
        }}
        onBack={() => setPending(null)}
      />
    )
  }

  return (
    <>
      <DropdownMenuGroup>
        {providers.map((provider) => (
          <DropdownMenuItem
            key={provider.id}
            closeOnClick={!asking.has(provider.id)}
            onClick={() =>
              asking.has(provider.id) ? setPending(provider) : onNewSession(provider.id, "")
            }
          >
            <ProviderIcon kind={provider.id} />
            {provider.name}
          </DropdownMenuItem>
        ))}
      </DropdownMenuGroup>
      <DropdownMenuSeparator />
      <DropdownMenuGroup>
        <DropdownMenuItem onClick={() => onNewSession("shell", "")}>
          <Terminal />
          {terminalLabel}
        </DropdownMenuItem>
        {run && <RunMenuItem run={run} />}
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
